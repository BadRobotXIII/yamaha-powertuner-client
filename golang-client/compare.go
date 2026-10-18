package powertuner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Compare reports exact byte changes and JSON-pointer value changes. Object key
// order/whitespace affect bytes only. Numbers retain their exact spelling.
func Compare(previous, current *Snapshot) (string, error) {
	if previous == nil || current == nil {
		return "", fmt.Errorf("two snapshots are required")
	}
	if previous.Source != current.Source {
		return "", fmt.Errorf("snapshots have different source/address/endpoint/options")
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Previous: %s\nCurrent:  %s\n", previous.CapturedAt.Format("2006-01-02T15:04:05.999Z07:00"), current.CapturedAt.Format("2006-01-02T15:04:05.999Z07:00"))
	old, now := map[string][]byte{}, map[string][]byte{}
	for _, p := range previous.Parts {
		old[p.Name] = p.Body
	}
	for _, p := range current.Parts {
		now[p.Name] = p.Body
	}
	keys := map[string]bool{}
	for k := range old {
		keys[k] = true
	}
	for k := range now {
		keys[k] = true
	}
	names := []string{}
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		a, aok := old[name]
		b, bok := now[name]
		fmt.Fprintf(&out, "\n[%s]\n", name)
		if !aok {
			out.WriteString("Response added\n")
			continue
		}
		if !bok {
			out.WriteString("Response removed\n")
			continue
		}
		if bytes.Equal(a, b) {
			out.WriteString("Unchanged (byte-for-byte)\n")
			continue
		}
		n := len(a)
		if len(b) > n {
			n = len(b)
		}
		count, shown := 0, 0
		var offsets strings.Builder
		for i := 0; i < n; i++ {
			if i < len(a) && i < len(b) && a[i] == b[i] {
				continue
			}
			count++
			if shown < 32 {
				fmt.Fprintf(&offsets, "  0x%08x: %s -> %s\n", i, byteAt(a, i), byteAt(b, i))
				shown++
			}
		}
		fmt.Fprintf(&out, "Bytes: %d -> %d; %d differing offsets\n", len(a), len(b), count)
		if count > shown {
			fmt.Fprintf(&offsets, "  ... %d further differing offsets\n", count-shown)
		}
		out.WriteString(offsets.String())
		if utf8.Valid(a) && utf8.Valid(b) && json.Valid(a) && json.Valid(b) {
			var av, bv any
			da := json.NewDecoder(bytes.NewReader(a))
			da.UseNumber()
			_ = da.Decode(&av)
			db := json.NewDecoder(bytes.NewReader(b))
			db.UseNumber()
			_ = db.Decode(&bv)
			changes := []string{}
			total := 0
			walkJSON("", av, bv, &changes, &total)
			if total == 0 {
				out.WriteString("JSON values unchanged (format/key-order differences only)\n")
			} else {
				fmt.Fprintf(&out, "JSON changes: %d\n%s", total, strings.Join(changes, ""))
				if total > len(changes) {
					fmt.Fprintf(&out, "... %d further JSON changes\n", total-len(changes))
				}
			}
		} else {
			out.WriteString("No JSON value comparison; raw bytes compared only.\n")
		}
	}
	return out.String(), nil
}
func byteAt(b []byte, i int) string {
	if i >= len(b) {
		return "<absent>"
	}
	return fmt.Sprintf("%02x", b[i])
}
func pointer(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }
func value(v any) string {
	b, _ := json.Marshal(v)
	if len(b) > 240 {
		return string(b[:240]) + "..."
	}
	return string(b)
}
func change(path, kind string, a, b any, lines *[]string, total *int) {
	*total++
	if len(*lines) < 200 {
		*lines = append(*lines, fmt.Sprintf("  %s %s: %s -> %s\n", kind, path, value(a), value(b)))
	}
}
func walkJSON(path string, a, b any, lines *[]string, total *int) {
	if reflect.DeepEqual(a, b) {
		return
	}
	if am, ok := a.(map[string]any); ok {
		if bm, ok := b.(map[string]any); ok {
			keys := map[string]bool{}
			for k := range am {
				keys[k] = true
			}
			for k := range bm {
				keys[k] = true
			}
			sorted := []string{}
			for k := range keys {
				sorted = append(sorted, k)
			}
			sort.Strings(sorted)
			for _, k := range sorted {
				av, ao := am[k]
				bv, bo := bm[k]
				p := path + "/" + pointer(k)
				if !ao {
					change(p, "added", nil, bv, lines, total)
				} else if !bo {
					change(p, "removed", av, nil, lines, total)
				} else {
					walkJSON(p, av, bv, lines, total)
				}
			}
			return
		}
	}
	if aa, ok := a.([]any); ok {
		if bb, ok := b.([]any); ok {
			n := len(aa)
			if len(bb) > n {
				n = len(bb)
			}
			for i := 0; i < n; i++ {
				p := path + "/" + strconv.Itoa(i)
				if i >= len(aa) {
					change(p, "added", nil, bb[i], lines, total)
				} else if i >= len(bb) {
					change(p, "removed", aa[i], nil, lines, total)
				} else {
					walkJSON(p, aa[i], bb[i], lines, total)
				}
			}
			return
		}
	}
	change(path, "changed", a, b, lines, total)
}

// History keeps only the latest two successful captures for each source. Call
// after a successful Capture only. It is owned by the GUI's main goroutine.
type History struct{ pairs map[string][2]*Snapshot }

func (h *History) Add(s *Snapshot) (*Snapshot, *Snapshot) {
	if h.pairs == nil {
		h.pairs = map[string][2]*Snapshot{}
	}
	pair := h.pairs[s.Source]
	pair = [2]*Snapshot{pair[1], s}
	h.pairs[s.Source] = pair
	return pair[0], pair[1]
}
