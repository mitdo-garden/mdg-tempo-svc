package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/models"
)

type TelegramRepository struct {
	db *gorm.DB
}

func NewTelegramRepository(db *gorm.DB) *TelegramRepository {
	return &TelegramRepository{db: db}
}

// ChatID returns the user's Telegram chat id, or empty when unset.
func (r *TelegramRepository) ChatID(ctx context.Context, userID string) (string, error) {
	var row models.UserTelegram
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.ChatID, nil
}

// SaveLinkCode stores a one-time code the bot can redeem into this user's chat id.
func (r *TelegramRepository) SaveLinkCode(ctx context.Context, userID, code string) error {
	row := models.UserTelegram{
		UserID:    userID,
		LinkCode:  code,
		UpdatedAt: time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"link_code", "updated_at"}),
	}).Create(&row).Error
}

// ClaimLink writes chatID onto the user who issued code. False when the code is unknown.
func (r *TelegramRepository) ClaimLink(ctx context.Context, code, chatID string) (bool, error) {
	code = strings.TrimSpace(code)
	chatID = strings.TrimSpace(chatID)
	if code == "" || chatID == "" {
		return false, nil
	}
	res := r.db.WithContext(ctx).Model(&models.UserTelegram{}).
		Where("link_code = ?", code).
		Updates(map[string]any{
			"chat_id":    chatID,
			"link_code":  "",
			"updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// SaveChatID stores the chat id for this user. An empty id clears it.
func (r *TelegramRepository) SaveChatID(ctx context.Context, userID, chatID string) error {
	row := models.UserTelegram{
		UserID:    userID,
		ChatID:    chatID,
		UpdatedAt: time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"chat_id", "updated_at"}),
	}).Create(&row).Error
}
