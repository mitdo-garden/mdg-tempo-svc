package config

import (
	"fmt"
	"os"
	"strings"
)

// ExpandString resolves Spring-style placeholders:
//
//	${NAME}           → env NAME, or "" if unset
//	${NAME:default}   → env NAME, or default if unset/empty
//
// Nested forms work, e.g. ${KMS_REGION:${AWS_REGION:us-east-1}}.
// Only the first ':' separates name from default (so defaults may contain ':').
func ExpandString(s string) string {
	for {
		start, end := findInnermostPlaceholder(s)
		if start < 0 {
			return s
		}
		inner := s[start+2 : end]
		name, def, hasDef := strings.Cut(inner, ":")
		name = strings.TrimSpace(name)
		val, ok := os.LookupEnv(name)
		if !ok || val == "" {
			if hasDef {
				val = def
			} else {
				val = ""
			}
		}
		s = s[:start] + val + s[end+1:]
	}
}

// findInnermostPlaceholder returns indices of the innermost ${...} pair.
// start points at '$', end points at matching '}'. Returns -1,-1 if none.
func findInnermostPlaceholder(s string) (start, end int) {
	start = -1
	for i := 0; i < len(s); i++ {
		if i+1 < len(s) && s[i] == '$' && s[i+1] == '{' {
			start = i
			i++ // skip '{' so a later '}' closes this placeholder
			continue
		}
		if start >= 0 && s[i] == '}' {
			return start, i
		}
	}
	return -1, -1
}

// expandSettings walks a viper AllSettings tree and expands string leaves.
func expandSettings(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = expandAny(v)
	}
	return out
}

func expandAny(v any) any {
	switch t := v.(type) {
	case string:
		return ExpandString(t)
	case map[string]any:
		return expandSettings(t)
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[fmt.Sprint(k)] = expandAny(val)
		}
		return m
	case []any:
		arr := make([]any, len(t))
		for i, el := range t {
			arr[i] = expandAny(el)
		}
		return arr
	default:
		return v
	}
}

// mergeExpanded writes an expanded settings tree back into viper (dotted keys).
func mergeExpanded(set func(key string, value any), prefix string, m map[string]any) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch child := v.(type) {
		case map[string]any:
			mergeExpanded(set, key, child)
		default:
			set(key, child)
		}
	}
}
