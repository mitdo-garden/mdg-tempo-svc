package models

// TempoTag is a lowercase token on a tempo.
type TempoTag struct {
	TempoID string `gorm:"primaryKey;size:36" json:"tempoId"`
	Tag     string `gorm:"primaryKey;size:64" json:"tag"`
}

func (TempoTag) TableName() string { return "tempo_tags" }

// TagCount is a GET /tags row.
type TagCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}
