package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ReviewHistory is one graded review on a tempo.
type ReviewHistory struct {
	ID         string    `gorm:"primaryKey;size:36" json:"id"`
	UserID     string    `gorm:"size:36;not null;index:idx_history_user" json:"userId"`
	TempoID    string    `gorm:"size:36;not null;index:idx_history_tempo" json:"tempoId"`
	Rating     int       `gorm:"not null" json:"rating"`
	ReviewedAt time.Time `gorm:"not null;index:idx_history_at" json:"reviewedAt"`
}

func (ReviewHistory) TableName() string { return "review_history" }

func (h *ReviewHistory) BeforeCreate(tx *gorm.DB) error {
	if h.ID == "" {
		h.ID = uuid.NewString()
	}
	return nil
}

// ActivityLog is the tracking log (Reviewed / Opened / Ignored).
type ActivityLog struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	UserID    string    `gorm:"size:36;not null;index:idx_activity_user" json:"userId"`
	TempoID   string    `gorm:"size:36;not null;index:idx_activity_tempo" json:"tempoId"`
	Channel   string    `gorm:"size:32;not null" json:"channel"` // push|email|telegram|slack|app
	Status    string    `gorm:"size:32;not null" json:"status"`  // Reviewed|Opened|Ignored|Reminded
	CreatedAt time.Time `gorm:"not null;index:idx_activity_at" json:"createdAt"`
}

const (
	ChannelTelegram = "telegram"
	StatusReminded  = "Reminded"
)

func (ActivityLog) TableName() string { return "activity_log" }

func (a *ActivityLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}
