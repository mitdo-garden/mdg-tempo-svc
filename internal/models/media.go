package models

import "time"

const (
	MediaImage = "image"
	MediaAudio = "audio"
	MediaVideo = "video"
)

// Media is an attachment on a tempo.
type Media struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	TempoID   string    `gorm:"size:36;not null;index:idx_media_tempo" json:"tempoId"`
	Kind      string    `gorm:"size:16;not null" json:"kind"`
	URL       string    `gorm:"size:2048;not null" json:"url"`
	Title     string    `gorm:"size:255;not null;default:''" json:"title"`
	SortOrder int       `gorm:"not null;default:0" json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
}

func (Media) TableName() string { return "media" }

func (m Media) ToDTO() MediaDTO {
	return MediaDTO{ID: m.ID, Kind: m.Kind, URL: m.URL, Title: m.Title}
}

func ValidMediaKind(kind string) bool {
	switch kind {
	case MediaImage, MediaAudio, MediaVideo:
		return true
	default:
		return false
	}
}
