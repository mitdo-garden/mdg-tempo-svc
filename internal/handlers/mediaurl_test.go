package handlers

import "testing"

func TestPublicMediaURL(t *testing.T) {
	const base = "http://localhost:8081/media"
	const name = "abc.png"
	if got := publicMediaURL(base, "localhost", name, true); got != "http://localhost:8081/media/abc.png" {
		t.Fatalf("localhost: %s", got)
	}
	if got := publicMediaURL(base, "192.168.88.165", name, true); got != "http://192.168.88.165:8081/media/abc.png" {
		t.Fatalf("lan: %s", got)
	}
	if got := publicMediaURL(base, "mitdo.tech", name, true); got != "http://mitdo.tech:8081/media/abc.png" {
		t.Fatalf("domain: %s", got)
	}
	if got := publicMediaURL(base, "192.168.88.165", name, false); got != "http://localhost:8081/media/abc.png" {
		t.Fatalf("prod keeps configured host: %s", got)
	}
}
