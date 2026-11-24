package server

import "testing"

func TestOriginAllowed(t *testing.T) {
	const frontend = "http://localhost:5173"
	cases := []struct {
		origin  string
		lan     bool
		allowed bool
	}{
		{"http://localhost:5173", false, true},
		{"http://192.168.88.165:5173", false, false},
		{"http://192.168.88.165:5173", true, true},
		{"http://127.0.0.1:5173", true, true},
		{"http://10.0.0.8:5173", true, true},
		{"http://192.168.88.165:5174", true, false},
		{"https://192.168.88.165:5173", true, false},
		{"http://mitdo.tech:5173", true, true},
		{"https://mitdo.tech", true, true},
		{"https://www.mitdo.tech", true, true},
		{"http://mitdo.tech:5173", false, false},
		{"http://evil.example:5173", true, false},
		{"", true, false},
	}
	for _, tc := range cases {
		if got := originAllowed(tc.origin, frontend, tc.lan); got != tc.allowed {
			t.Errorf("origin %q lan=%v: got %v want %v", tc.origin, tc.lan, got, tc.allowed)
		}
	}
}
