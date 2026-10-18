package powertuner

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func snap(body string) *Snapshot {
	return &Snapshot{Version: 1, Source: "mock|monitor", CapturedAt: time.Unix(1, 0).UTC(), Parts: []Part{{"monitor", []byte(body)}}}
}
func TestSnapshotRoundTripBinary(t *testing.T) {
	s := snap("")
	s.Parts[0].Body = make([]byte, 256)
	for i := range s.Parts[0].Body {
		s.Parts[0].Body[i] = byte(i)
	}
	var b bytes.Buffer
	if err := WriteSnapshot(&b, s); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadSnapshot(&b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, restored) {
		t.Fatal("snapshot bytes/metadata changed")
	}
	for _, input := range []string{`{}`, `{"version":2,"source":"x","parts":[{"name":"x"}]}`, `{"version":1,"source":"x","parts":[{"name":"x"},{"name":"x"}]}`} {
		if _, err := ReadSnapshot(strings.NewReader(input)); err == nil {
			t.Fatal("accepted invalid snapshot")
		}
	}
}
func TestCompareJSONAndBytes(t *testing.T) {
	for _, tc := range []struct{ before, after, want string }{
		{`{"EG_mode":0}`, `{"EG_mode":1}`, `changed /EG_mode: 0 -> 1`},
		{`{"x":9007199254740993}`, `{"x":9007199254740994}`, `9007199254740993 -> 9007199254740994`},
		{`{"a":1,"b":2}`, "{\n\"b\":2,\"a\":1}", "JSON values unchanged"},
		{`{}`, `{"a/b~":null}`, `added /a~1b~0`},
		{`{"a":[1,2]}`, `{"a":[1]}`, `removed /a/1: 2 -> null`},
		{"\x00\xff", "\x00\xfe\x01", "2 differing offsets"},
		{`{"x":1.0}`, `{"x":1}`, `changed /x: 1.0 -> 1`},
		{`{}`, `{}`, `Unchanged (byte-for-byte)`},
	} {
		report, err := Compare(snap(tc.before), snap(tc.after))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(report, tc.want) {
			t.Fatalf("expected %q in %s", tc.want, report)
		}
	}
	a, b := snap("a"), snap("b")
	b.Source = "different bike"
	if _, err := Compare(a, b); err == nil {
		t.Fatal("compared different sources")
	}
}
func TestHistorySuccessiveSources(t *testing.T) {
	var h History
	a, b, c := snap("a"), snap("b"), snap("c")
	other := snap("other")
	other.Source = "other"
	p, _ := h.Add(a)
	if p != nil {
		t.Fatal("unexpected baseline")
	}
	h.Add(other)
	p, n := h.Add(b)
	if p != a || n != b {
		t.Fatal("wrong baseline")
	}
	p, n = h.Add(c)
	if p != b || n != c {
		t.Fatal("did not advance to previous capture")
	}
}
func TestFormattingPreservesOriginal(t *testing.T) {
	b := []byte{0, 1, 15, 127, 128, 255}
	original := append([]byte(nil), b...)
	if !strings.Contains(FormatBody(b, "hex", 0), "00 01 0f 7f 80 ff") {
		t.Fatal("wrong hex")
	}
	if !strings.Contains(FormatBody(b, "binary", 0), "00000000 00000001 00001111 01111111 10000000 11111111") {
		t.Fatal("wrong bits")
	}
	if !strings.Contains(FormatBody(b, "all", 0), "not valid UTF-8") {
		t.Fatal("invalid UTF-8 silently changed")
	}
	if !bytes.Equal(b, original) {
		t.Fatal("format changed original bytes")
	}
	if FormatBody(b, "raw", 0) != string(b) {
		t.Fatal("raw bytes changed")
	}
	if !strings.Contains(FormatBody(b, "hex", 2), "first 2 of 6") {
		t.Fatal("preview not marked")
	}
	if !strings.Contains(FormatBody([]byte(`{"n":9007199254740993}`), "json", 0), "9007199254740993") {
		t.Fatal("numeric precision lost")
	}
	s := snap("{}")
	s.Parts = append(s.Parts, Part{"ecu", b})
	if _, err := FormatSnapshot(s, "raw", "", 0); err == nil {
		t.Fatal("ambiguous raw export accepted")
	}
	out, err := FormatSnapshot(s, "raw", "ecu", 0)
	if err != nil || out != string(b) {
		t.Fatal("selected raw export changed")
	}
}
func TestCapturePreservesNonJSONMapBodyGETOnly(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/info/" {
			fmt.Fprint(w, `{"SerialNumber":"mock"}`)
		} else {
			w.Write([]byte{0, 255, 1})
		}
	}))
	defer server.Close()
	c, err := New(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	s, err := c.Capture(context.Background(), Request{Command: "maps", IncludeEPS: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Parts) != 3 || !bytes.Equal(s.Parts[1].Body, []byte{0, 255, 1}) {
		t.Fatal("binary map body changed")
	}
	if !reflect.DeepEqual(calls, []string{"GET /info/", "GET /ecu/settings/", "GET /eps/settings/"}) {
		t.Fatal(calls)
	}
}
