package powertuner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMapSequenceGETOnly(t *testing.T) {
	var got []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.RequestURI())
		if r.Body != nil && r.ContentLength != 0 {
			t.Error("unexpected request body")
		}
		switch r.URL.Path {
		case "/info/":
			fmt.Fprint(w, `{"SerialNumber":"mock-1"}`)
		case "/ecu/settings/":
			fmt.Fprint(w, `{"ecusettings":[{"FI":{"rows":[[-7,1,2,3]]}}]}`)
		case "/eps/settings/":
			fmt.Fprint(w, `{"epssettings":[]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c, err := New(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	maps, err := c.DownloadMaps(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(maps.ECU), "[-7,1,2,3]") {
		t.Fatal("map values changed")
	}
	want := []string{"GET /info/", "GET /ecu/settings/", "GET /eps/settings/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v; want %v", got, want)
	}
}

func TestFailuresDoNotTriggerExtraRequests(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"missing serial", 200, `{}`}, {"bad JSON", 200, `broken`},
		{"authentication required", 403, ``}, {"redirect", 302, ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Location", "/unexpected/")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer s.Close()
			c, err := New(s.URL, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			if _, err := c.DownloadMaps(context.Background(), true); err == nil {
				t.Fatal("expected failure")
			}
			if requests != 1 {
				t.Fatalf("sent %d requests", requests)
			}
		})
	}
}

func TestResourceAllowlistAndLogQuery(t *testing.T) {
	requests := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "GET" || r.URL.RequestURI() != "/ecu/log/?request=3" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		fmt.Fprint(w, `{"EcuLog":[]}`)
	}))
	defer s.Close()
	c, err := New(s.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for _, resource := range []Resource{"/ecu/pass/", "fuel", "../key", "maps?write=1"} {
		if _, err := c.Read(context.Background(), resource); err == nil {
			t.Errorf("accepted %q", resource)
		}
	}
	for _, page := range []int{-1, 4} {
		if _, err := c.ActivityLog(context.Background(), page); err == nil {
			t.Error("accepted bad page")
		}
	}
	if requests != 0 {
		t.Fatal("invalid input sent requests")
	}
	if _, err := c.ActivityLog(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
}

func TestResponseLimitAndCancellation(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, strings.Repeat("x", maxBodyBytes+1)) }))
	defer s.Close()
	c, err := New(s.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Read(context.Background(), Info); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size limit, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Read(ctx, Info); err == nil {
		t.Fatal("expected cancellation")
	}
}

func TestInvalidConfiguration(t *testing.T) {
	for _, addr := range []string{"ftp://host", "http://", "http://user:pass@host", "http://host/path", "http://host?x=1", "http://host#fragment"} {
		if _, err := New(addr, time.Second); err == nil {
			t.Errorf("accepted %q", addr)
		}
	}
	if _, err := New(DefaultAddress, 0); err == nil {
		t.Fatal("accepted zero timeout")
	}
}
