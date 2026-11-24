package models

import (
	"crypto/rand"
	"regexp"
	"strings"
	"time"
)

// Telegram chat ids are signed integers (private chats positive, groups negative).
var telegramChatID = regexp.MustCompile(`^-?[1-9]\d{0,19}$`)

// UserTelegram is one garden user's Telegram destination.
type UserTelegram struct {
	UserID    string    `gorm:"primaryKey;size:36" json:"-"`
	ChatID    string    `gorm:"column:chat_id;size:32;not null" json:"chatId"`
	LinkCode  string    `gorm:"column:link_code;size:16;not null" json:"-"`
	UpdatedAt time.Time `json:"-"`
}

func (UserTelegram) TableName() string { return "user_telegram" }

// ValidTelegramChatID accepts an empty value (clear the setting) or a numeric chat id.
func ValidTelegramChatID(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	return telegramChatID.MatchString(s)
}

// NewTelegramLinkCode is the token sent to the bot as /start <code>.
func NewTelegramLinkCode() (string, error) {
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, len(buf))
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}
