package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/models"
)

type HistoryRepository struct {
	db *gorm.DB
}

func NewHistoryRepository(db *gorm.DB) *HistoryRepository {
	return &HistoryRepository{db: db}
}

func (r *HistoryRepository) AppendReview(ctx context.Context, userID string, tempoID string, rating int, at time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		h := models.ReviewHistory{
			UserID:     userID,
			TempoID:    tempoID,
			Rating:     rating,
			ReviewedAt: at,
		}
		if err := tx.Create(&h).Error; err != nil {
			return err
		}
		log := models.ActivityLog{
			UserID:    userID,
			TempoID:   tempoID,
			Channel:   "app",
			Status:    "Reviewed",
			CreatedAt: at,
		}
		return tx.Create(&log).Error
	})
}

// AppendReminder records that a tempo was pushed on Telegram.
func (r *HistoryRepository) AppendReminder(ctx context.Context, userID string, tempoID string, at time.Time) error {
	log := models.ActivityLog{
		UserID:    userID,
		TempoID:   tempoID,
		Channel:   models.ChannelTelegram,
		Status:    models.StatusReminded,
		CreatedAt: at.UTC(),
	}
	return r.db.WithContext(ctx).Create(&log).Error
}
