package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// mergeEnvAndProfiles merges config.<env>.yaml, then any extra profiles from
// app.profiles in that file and APP_PROFILES (comma-separated), like Spring's
// continuous / included profiles.
func mergeEnvAndProfiles(v *viper.Viper, env string) error {
	if err := mergeNamedConfig(v, "config."+env); err != nil {
		return fmt.Errorf("read configs/config.%s.yaml: %w", env, err)
	}

	seen := map[string]struct{}{env: {}}
	var queue []string
	queue = append(queue, stringSlice(v.Get("app.profiles"))...)
	queue = append(queue, splitCSV(os.Getenv("APP_PROFILES"))...)

	for len(queue) > 0 {
		p := strings.ToLower(strings.TrimSpace(queue[0]))
		queue = queue[1:]
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		if err := mergeNamedConfig(v, "config."+p); err != nil {
			return fmt.Errorf("read configs/config.%s.yaml (profile): %w", p, err)
		}
		queue = append(queue, stringSlice(v.Get("app.profiles"))...)
	}
	return nil
}

func mergeNamedConfig(v *viper.Viper, name string) error {
	v.SetConfigName(name)
	return v.MergeInConfig()
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func stringSlice(raw any) []string {
	switch t := raw.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, el := range t {
			s := strings.TrimSpace(fmt.Sprint(el))
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	case string:
		return splitCSV(t)
	default:
		return nil
	}
}
