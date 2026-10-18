// Package powertuner provides GET-only access to endpoints found in Power Tuner 2.5.0.
package powertuner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const DefaultAddress = "http://192.168.0.1:8080"
const maxBodyBytes = 4 << 20

// Resource is a fixed, statically observed read endpoint, not an arbitrary URL.
type Resource string

const (
	Info    Resource = "info"
	Maps    Resource = "maps"
	EPS     Resource = "eps"
	Monitor Resource = "monitor"
	Model   Resource = "model"
	Lock    Resource = "lock"
	Laps    Resource = "laps"
)

var paths = map[Resource]string{
	Info: "/info/", Maps: "/ecu/settings/", EPS: "/eps/settings/",
	Monitor: "/ecu/monitor/", Model: "/ecu/model/", Lock: "/ecu/lock/",
	Laps: "/ecu/lap/",
}

type Client struct {
	base url.URL
	http *http.Client
}

// New creates a direct HTTP client. It does not contact the server.
// Redirects and environment proxies are disabled to keep requests on the chosen host.
func New(address string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
		u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return nil, fmt.Errorf("base URL must be http(s)://host[:port], without credentials, path, query, or fragment")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be positive")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	// Avoid Go's transparent decompression; preserve response payload bytes.
	transport.DisableCompression = true
	// Fresh connections also prevent net/http's retry of GETs on stale pooled connections.
	transport.DisableKeepAlives = true
	return &Client{base: *u, http: &http.Client{
		Timeout: timeout, Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *Client) Close() { c.http.CloseIdleConnections() }

// Read issues one GET with no body. Returned bytes retain the wire values;
// no vehicle-specific axis, offset, sentinel, or unit conversion is applied.
func (c *Client) Read(ctx context.Context, resource Resource) ([]byte, error) {
	path, ok := paths[resource]
	if !ok {
		return nil, fmt.Errorf("unknown read resource %q", resource)
	}
	return c.get(ctx, path, "")
}

// ActivityLog reads one of the four activity-log pages observed in the APK.
func (c *Client) ActivityLog(ctx context.Context, page int) ([]byte, error) {
	if page < 0 || page > 3 {
		return nil, fmt.Errorf("log page must be between 0 and 3")
	}
	return c.get(ctx, "/ecu/log/", fmt.Sprintf("request=%d", page))
}

func (c *Client) get(ctx context.Context, path, query string) ([]byte, error) {
	u := c.base
	u.Path, u.RawPath, u.RawQuery = path, "", query
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Charset", "UTF-8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MSIE 6.0; WIndows NT 5.1; SV1)")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %s (no authentication or retry attempted)", path, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(body) > maxBodyBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", maxBodyBytes)
	}
	return body, nil
}

// MapDownload retains the raw JSON from each step of the APK's receive sequence.
type MapDownload struct {
	Info json.RawMessage `json:"info"`
	ECU  json.RawMessage `json:"ecu"`
	EPS  json.RawMessage `json:"eps,omitempty"`
}

// DownloadMaps reads identity, then ECU maps, optionally followed by EPS maps.
// It does not save/overwrite an ECU map or perform password/key exchange.
func (c *Client) DownloadMaps(ctx context.Context, includeEPS bool) (*MapDownload, error) {
	info, err := c.Read(ctx, Info)
	if err != nil {
		return nil, err
	}
	var identity struct{ SerialNumber string }
	if err := json.Unmarshal(info, &identity); err != nil {
		return nil, fmt.Errorf("parse info: %w", err)
	}
	if identity.SerialNumber == "" {
		return nil, fmt.Errorf("info response has no SerialNumber; map download stopped")
	}
	ecu, err := c.Read(ctx, Maps)
	if err != nil {
		return nil, err
	}
	if !json.Valid(ecu) {
		return nil, fmt.Errorf("ECU map response is not valid JSON")
	}
	result := &MapDownload{Info: info, ECU: ecu}
	if includeEPS {
		eps, err := c.Read(ctx, EPS)
		if err != nil {
			return nil, err
		}
		if !json.Valid(eps) {
			return nil, fmt.Errorf("EPS map response is not valid JSON")
		}
		result.EPS = eps
	}
	return result, nil
}
