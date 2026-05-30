package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseAssignBody parses a `key = value` assignment block into a nested map
// ready for json.Marshal → json.Unmarshal into a target struct.
//
// Lines beginning with # and blank lines are skipped.
// Keys are dotted paths: trigger.github.events = ["issues.labeled"]
func parseAssignBody(body, context string) (map[string]any, error) {
	m := map[string]any{}
	for i, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, err := parseAssignLine(line)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", context, i+1, err)
		}
		if err := setNestedPath(m, key, val); err != nil {
			return nil, fmt.Errorf("%s: key %q: %w", context, key, err)
		}
	}
	return m, nil
}

func parseAssignLine(line string) (key string, value any, err error) {
	eqIdx := findTopLevelEq(line)
	if eqIdx < 0 {
		return "", nil, fmt.Errorf("expected `key = value`, got %q", line)
	}
	key = strings.TrimSpace(line[:eqIdx])
	if key == "" {
		return "", nil, fmt.Errorf("empty key in %q", line)
	}
	rvalStr := strings.TrimSpace(line[eqIdx+1:])
	if rvalStr == "" {
		return "", nil, fmt.Errorf("empty value for key %q", key)
	}
	value, err = parseRvalue(rvalStr)
	if err != nil {
		return "", nil, fmt.Errorf("key %q: %w", key, err)
	}
	return key, value, nil
}

// findTopLevelEq returns the index of the first `=` outside a double-quoted
// string, or -1 if none found.
func findTopLevelEq(s string) int {
	inQuote := false
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' && inQuote {
			escape = true
			continue
		}
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if c == '=' && !inQuote {
			return i
		}
	}
	return -1
}

// parseRvalue parses the right-hand side of an assignment.
// Supports: JSON strings, arrays, objects, booleans, null, numbers,
// and bareword strings (returned as-is; references resolved in a later phase).
func parseRvalue(s string) (any, error) {
	switch s[0] {
	case '"', '[', '{':
		var v any
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return nil, fmt.Errorf("invalid value %q: %w", s, err)
		}
		return v, nil
	}
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	}
	var n json.Number
	if err := json.Unmarshal([]byte(s), &n); err == nil {
		if f, err2 := n.Float64(); err2 == nil {
			return f, nil
		}
	}
	return s, nil
}

// setNestedPath sets a dotted path in a nested map, creating intermediate
// maps as needed.
func setNestedPath(m map[string]any, path string, value any) error {
	idx := strings.IndexByte(path, '.')
	if idx < 0 {
		m[path] = value
		return nil
	}
	key, rest := path[:idx], path[idx+1:]
	sub, exists := m[key]
	if !exists {
		sub = map[string]any{}
		m[key] = sub
	}
	subMap, ok := sub.(map[string]any)
	if !ok {
		return fmt.Errorf("path conflict: %q is already a scalar value", key)
	}
	return setNestedPath(subMap, rest, value)
}
