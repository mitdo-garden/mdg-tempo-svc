package handlers

import (
	"net"
	"net/url"
	"strings"
)

// publicMediaURL returns the stored file URL. In dev, a localhost base is
// rewritten to the host that actually received the request, so a client using
// the LAN IP loads /media from that IP instead of localhost.
func publicMediaURL(base, requestHost, name string, rewriteLocal bool) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	name = strings.TrimLeft(name, "/")
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return base + "/" + name
	}
	if rewriteLocal && shareableHost(u.Hostname()) && shareableHost(requestHost) {
		host := requestHost
		if h, _, err := net.SplitHostPort(requestHost); err == nil {
			host = h
		}
		host = strings.Trim(host, "[]")
		if port := u.Port(); port != "" {
			u.Host = net.JoinHostPort(host, port)
		} else {
			u.Host = host
		}
	}
	return strings.TrimRight(u.String(), "/") + "/" + name
}

func shareableHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "mitdo.tech" || strings.HasSuffix(host, ".mitdo.tech") {
		return true
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return net.ParseIP(host) != nil
}
