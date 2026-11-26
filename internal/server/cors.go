package server

import (
	"net"
	"net/url"
	"strings"
)

// originAllowed accepts the configured frontend origin plus Mitdo public domains.
// In dev it also accepts the same scheme/port on localhost or LAN hosts.
func originAllowed(origin, frontendURL string, allowLAN bool) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	frontendURL = strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	if origin == "" {
		return false
	}
	if frontendURL != "" && origin == frontendURL {
		return true
	}

	got, err := url.Parse(origin)
	if err != nil || got.Hostname() == "" {
		return false
	}
	if onPublicDomain(got.Hostname()) {
		if strings.EqualFold(got.Scheme, "https") {
			return true
		}
		if allowLAN && strings.EqualFold(got.Scheme, "http") {
			return true
		}
		return false
	}

	if !allowLAN || frontendURL == "" {
		return false
	}
	want, err := url.Parse(frontendURL)
	if err != nil || want.Scheme == "" {
		return false
	}
	if !strings.EqualFold(got.Scheme, want.Scheme) || got.Port() != want.Port() {
		return false
	}
	return lanHost(got.Hostname())
}

func onPublicDomain(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	for _, domain := range []string{"mitdo.cloud", "mitdo.tech"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func lanHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return net.ParseIP(host) != nil
}
