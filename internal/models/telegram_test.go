package models

import "testing"

func TestValidTelegramChatID(t *testing.T) {
	ok := []string{"", "  ", "123456789", "-1001234567890"}
	bad := []string{"0", "-0", "0123", "abc", "12 34", "+123", "1.5"}
	for _, s := range ok {
		if !ValidTelegramChatID(s) {
			t.Fatalf("want valid %q", s)
		}
	}
	for _, s := range bad {
		if ValidTelegramChatID(s) {
			t.Fatalf("want invalid %q", s)
		}
	}
}
