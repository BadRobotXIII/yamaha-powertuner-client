package powertuner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Part contains one original HTTP response body. JSON persistence uses base64,
// so invalid UTF-8 and binary bytes round-trip without loss.
type Part struct {
	Name string `json:"name"`
	Body []byte `json:"body"`
}
type Snapshot struct {
	Version    int       `json:"version"`
	Source     string    `json:"source"`
	CapturedAt time.Time `json:"captured_at"`
	Parts      []Part    `json:"parts"`
}
type Request struct {
	Command    string
	IncludeEPS bool
	Page       int
}

func (r Request) Validate() error {
	if r.Command != "log" {
		if _, ok := paths[Resource(r.Command)]; !ok {
			return fmt.Errorf("unknown command %q", r.Command)
		}
	}
	if r.IncludeEPS && r.Command != "maps" {
		return fmt.Errorf("EPS option is only valid for maps")
	}
	if r.Page < 0 || r.Page > 3 || (r.Page != 0 && r.Command != "log") {
		return fmt.Errorf("log page must be 0..3 and used only with log")
	}
	return nil
}

// Capture is manual, one-shot, GET-only. On error it returns any completed parts;
// callers must not promote a partial capture into the comparison history.
func (c *Client) Capture(ctx context.Context, r Request) (*Snapshot, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	base := c.base
	base.Path = ""
	s := &Snapshot{Version: 1, Source: fmt.Sprintf("%s|%s|eps=%t|page=%d", base.String(), r.Command, r.IncludeEPS, r.Page), CapturedAt: time.Now().UTC()}
	read := func(name string, resource Resource) error {
		b, e := c.Read(ctx, resource)
		if e == nil {
			s.Parts = append(s.Parts, Part{name, b})
		}
		return e
	}
	if r.Command == "maps" {
		if err := read("info", Info); err != nil {
			return s, err
		}
		var identity struct{ SerialNumber string }
		if err := json.Unmarshal(s.Parts[0].Body, &identity); err != nil {
			return s, fmt.Errorf("parse info: %w", err)
		}
		if identity.SerialNumber == "" {
			return s, fmt.Errorf("info response has no SerialNumber; map download stopped")
		}
		if err := read("ecu", Maps); err != nil {
			return s, err
		}
		if r.IncludeEPS {
			if err := read("eps", EPS); err != nil {
				return s, err
			}
		}
		return s, nil
	}
	if r.Command == "log" {
		b, e := c.ActivityLog(ctx, r.Page)
		if e == nil {
			s.Parts = append(s.Parts, Part{"log", b})
		}
		return s, e
	}
	return s, read(r.Command, Resource(r.Command))
}
func WriteSnapshot(w io.Writer, s *Snapshot) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(s)
}
func ReadSnapshot(r io.Reader) (*Snapshot, error) {
	// Three base64-encoded 4 MiB bodies plus metadata fit well inside 20 MiB.
	b, err := io.ReadAll(io.LimitReader(r, 20<<20+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 20<<20 {
		return nil, fmt.Errorf("snapshot file exceeds 20 MiB")
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if s.Version != 1 || s.Source == "" || len(s.Parts) < 1 || len(s.Parts) > 3 {
		return nil, fmt.Errorf("invalid snapshot format")
	}
	seen := map[string]bool{}
	for _, p := range s.Parts {
		if p.Name == "" || seen[p.Name] || len(p.Body) > maxBodyBytes {
			return nil, fmt.Errorf("invalid snapshot response")
		}
		seen[p.Name] = true
	}
	return &s, nil
}
