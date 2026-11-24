package models

import "time"

// Example is a review prompt attached to a tempo.
type Example struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	TempoID   string    `gorm:"size:36;not null;index:idx_examples_tempo" json:"tempoId"`
	Text      string    `gorm:"type:text;not null" json:"text"`
	SortOrder int       `gorm:"not null;default:0" json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
}

func (Example) TableName() string { return "examples" }

func (e Example) ToDTO() ExampleDTO {
	return ExampleDTO{ID: e.ID, Text: e.Text}
}
