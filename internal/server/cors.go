package server

import (
	"net"
	"net/url"
	"strings"
)

const publicDomain = "mitdo.tech"

// originAllowed accepts the configured frontend origin. In dev it also accepts
// mitdo.tech (any port) and the same scheme and port on localhost or any IP.
func originAllowed(origin, frontendURL string, allowLAN bool) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	frontendURL = strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	if origin == "" || frontendURL == "" {
		return false
	}
	if origin == frontendURL {
		return true
	}
	if !allowLAN {
		return false
	}
	want, err := url.Parse(frontendURL)
	got, err2 := url.Parse(origin)
	if err != nil || err2 != nil || want.Scheme == "" || got.Hostname() == "" {
		return false
	}
	if onPublicDomain(got.Hostname()) {
		return true
	}
	if !strings.EqualFold(got.Scheme, want.Scheme) || got.Port() != want.Port() {
		return false
	}
	return lanHost(got.Hostname())
}

func onPublicDomain(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	return host == publicDomain || strings.HasSuffix(host, "."+publicDomain)
}

func lanHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return net.ParseIP(host) != nil
}
