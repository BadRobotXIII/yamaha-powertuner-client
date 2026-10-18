package powertuner

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

func ValidFormat(mode string) bool {
	switch mode {
	case "json", "text", "binary", "hex", "all", "raw":
		return true
	}
	return false
}

// FormatBody formats original response bytes. limit=0 means no preview limit.
func FormatBody(body []byte, mode string, limit int) string {
	total := len(body)
	if limit > 0 && len(body) > limit {
		body = body[:limit]
	}
	text := func() string {
		if !utf8.Valid(body) {
			return "[Preview is not valid UTF-8; use Hex/Binary or save original bytes.]"
		}
		var pretty bytes.Buffer
		if mode != "text" && json.Indent(&pretty, body, "", "  ") == nil {
			return pretty.String()
		}
		return string(body)
	}
	bits := func() string {
		var out strings.Builder
		for i, b := range body {
			if i%8 == 0 {
				if i > 0 {
					out.WriteByte('\n')
				}
				fmt.Fprintf(&out, "%08x: ", i)
			} else {
				out.WriteByte(' ')
			}
			fmt.Fprintf(&out, "%08b", b)
		}
		return out.String()
	}
	var out string
	switch mode {
	case "json", "text":
		out = text()
	case "hex":
		out = hex.Dump(body)
	case "binary":
		out = bits()
	case "all":
		out = "JSON / UTF-8 text\n" + text() + "\n\nBINARY (8 bits per byte)\n" + bits() + "\n\nHEX\n" + hex.Dump(body)
	case "raw":
		out = string(body)
	default:
		return "Unknown format"
	}
	if len(body) < total {
		out += fmt.Sprintf("\n[Preview: first %d of %d bytes; export raw response for full data.]", len(body), total)
	}
	return out
}
func FormatSnapshot(s *Snapshot, mode, part string, limit int) (string, error) {
	if !ValidFormat(mode) {
		return "", fmt.Errorf("unknown format %q", mode)
	}
	if part != "" {
		for _, p := range s.Parts {
			if p.Name == part {
				return FormatBody(p.Body, mode, limit), nil
			}
		}
		return "", fmt.Errorf("response %q not in snapshot", part)
	}
	if len(s.Parts) == 1 {
		return FormatBody(s.Parts[0].Body, mode, limit), nil
	}
	if mode == "raw" {
		return "", fmt.Errorf("raw output requires -part for multi-response maps")
	}
	// Retain the existing maps JSON bundle when all response bodies are JSON.
	if mode == "json" {
		bundle := map[string]json.RawMessage{}
		valid := true
		for _, p := range s.Parts {
			valid = valid && utf8.Valid(p.Body) && json.Valid(p.Body)
			bundle[p.Name] = p.Body
		}
		if valid {
			b, _ := json.Marshal(bundle)
			return FormatBody(b, mode, limit), nil
		}
	}
	var out strings.Builder
	for _, p := range s.Parts {
		fmt.Fprintf(&out, "=== %s (%d bytes) ===\n%s\n\n", p.Name, len(p.Body), FormatBody(p.Body, mode, limit))
	}
	return out.String(), nil
}
