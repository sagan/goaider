package datautil

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"

	"github.com/sagan/goaider/constants"
)

// DiffResult holds the structured diff.
// Internally it stores the same "diff map" representation you already had:
//
//   - map[string]any for objects
//   - map[int]any for slice/array differences
//   - leaf maps like {"from": x, "to": y}, {"added": x}, {"removed": x}
//
// JSON representation of DiffResult is exactly this structure.
type DiffResult struct {
	data any
}

// Diff computes the diff between a and b.
// If there are no differences, it returns nil.
func Diff(a, b any) *DiffResult {
	data := diffValue(a, b)
	if data == nil {
		return nil
	}
	return &DiffResult{data: data}
}

// Empty reports whether there are no differences.
func (d *DiffResult) Empty() bool {
	return d == nil || d.data == nil
}

// MarshalJSON makes the default JSON representation of DiffResult
// be the underlying diff map/tree.
func (d *DiffResult) MarshalJSON() ([]byte, error) {
	if d == nil || d.data == nil {
		return []byte(constants.NULL), nil
	}
	return json.Marshal(d.data)
}

// Print writes a human-readable diff to the given writer.
//
// Format examples (paths):
//
//	~ age: 30 -> 31
//	+ meta.zip = 90001
//	- tags[2] = oldValue
func (d *DiffResult) Print(w io.Writer) error {
	if d == nil || d.data == nil {
		_, err := fmt.Fprintln(w, "no differences")
		return err
	}
	return printNode(w, d.data, "")
}

//
// ---------- internal diff implementation ----------
//

func diffValue(a, b any) any {
	// No diff
	if reflect.DeepEqual(a, b) {
		return nil
	}

	// Map handling (map with string keys)
	if aMap, okA := toStringMap(a); okA {
		if bMap, okB := toStringMap(b); okB {
			return diffMaps(aMap, bMap)
		}
	}

	// Slice/array handling
	if a != nil && b != nil {
		ta := reflect.TypeOf(a)
		tb := reflect.TypeOf(b)
		if ta.Kind() == reflect.Slice && tb.Kind() == reflect.Slice {
			return diffSlices(a, b)
		}
	}

	// Fallback: simple value change
	return map[string]any{
		"from": a,
		"to":   b,
	}
}

// toStringMap converts v to map[string]any if it's a map with string keys.
func toStringMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}

	// Common fast paths
	switch m := v.(type) {
	case map[string]any:
		return m, true
	}

	rv := reflect.ValueOf(v)
	rt := rv.Type()
	if rt.Kind() != reflect.Map || rt.Key().Kind() != reflect.String {
		return nil, false
	}

	out := make(map[string]any, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		out[iter.Key().String()] = iter.Value().Interface()
	}
	return out, true
}

func diffMaps(a, b map[string]any) map[string]any {
	out := make(map[string]any)

	// Keys in a
	for k, aVal := range a {
		bVal, exists := b[k]
		if !exists {
			out[k] = map[string]any{"removed": aVal}
			continue
		}

		sub := diffValue(aVal, bVal)
		if sub != nil {
			out[k] = sub
		}
	}

	// Keys added in b
	for k, bVal := range b {
		if _, exists := a[k]; !exists {
			out[k] = map[string]any{"added": bVal}
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func diffSlices(a, b any) any {
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)

	maxLen := aVal.Len()
	if bVal.Len() > maxLen {
		maxLen = bVal.Len()
	}

	out := make(map[int]any)

	for i := 0; i < maxLen; i++ {
		var aItem, bItem any
		if i < aVal.Len() {
			aItem = aVal.Index(i).Interface()
		}
		if i < bVal.Len() {
			bItem = bVal.Index(i).Interface()
		}

		if i >= aVal.Len() {
			out[i] = map[string]any{"added": bItem}
			continue
		}
		if i >= bVal.Len() {
			out[i] = map[string]any{"removed": aItem}
			continue
		}

		sub := diffValue(aItem, bItem)
		if sub != nil {
			out[i] = sub
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

//
// ---------- printing helpers ----------
//

func printNode(w io.Writer, node any, path string) error {
	switch v := node.(type) {
	case map[string]any:
		// Leaf diff like {"from": ..., "to": ...} or {"added": ...} or {"removed": ...}
		if isLeafDiffMap(v) {
			return printLeafDiff(w, v, path)
		}

		// Nested object diff
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			childPath := joinPath(path, k)
			if err := printNode(w, v[k], childPath); err != nil {
				return err
			}
		}
	case map[int]any:
		// Slice/array diff
		indices := make([]int, 0, len(v))
		for i := range v {
			indices = append(indices, i)
		}
		sort.Ints(indices)
		for _, i := range indices {
			childPath := joinIndex(path, i)
			if err := printNode(w, v[i], childPath); err != nil {
				return err
			}
		}
	default:
		// Fallback: shouldn't really happen, but print something sane
		_, err := fmt.Fprintf(w, "%s = %v\n", path, v)
		return err
	}
	return nil
}

func isLeafDiffMap(m map[string]any) bool {
	allowed := map[string]struct{}{
		"from":    {},
		"to":      {},
		"added":   {},
		"removed": {},
	}
	hasOp := false

	for k := range m {
		if _, ok := allowed[k]; !ok {
			return false
		}
		hasOp = true
	}
	return hasOp
}

func printLeafDiff(w io.Writer, m map[string]any, path string) error {
	if v, ok := m["added"]; ok && len(m) == 1 {
		_, err := fmt.Fprintf(w, "+ %s = %v\n", path, v)
		return err
	}
	if v, ok := m["removed"]; ok && len(m) == 1 {
		_, err := fmt.Fprintf(w, "- %s = %v\n", path, v)
		return err
	}
	from, hasFrom := m["from"]
	to, hasTo := m["to"]
	if hasFrom && hasTo {
		_, err := fmt.Fprintf(w, "~ %s: %v -> %v\n", path, from, to)
		return err
	}

	// Fallback in weird cases
	_, err := fmt.Fprintf(w, "%s = %v\n", path, m)
	return err
}

func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func joinIndex(parent string, idx int) string {
	if parent == "" {
		return fmt.Sprintf("[%d]", idx)
	}
	return fmt.Sprintf("%s[%d]", parent, idx)
}
