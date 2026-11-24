package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ReminderChannels mirrors the UI reminder.channels object.
type ReminderChannels struct {
	Push     bool `json:"push"`
	Email    bool `json:"email"`
	Telegram bool `json:"telegram"`
	Slack    bool `json:"slack"`
}

// Reminder is stored as JSON on the tempo.
type Reminder struct {
	Channels ReminderChannels `json:"channels"`
	Mode     string           `json:"mode"` // auto | daily | weekly
	Time     string           `json:"time"` // HH:mm
	Fun      bool             `json:"fun"`
}

func DefaultReminder() Reminder {
	return Reminder{
		Channels: ReminderChannels{Push: true},
		Mode:     "auto",
		Time:     "08:00",
		Fun:      true,
	}
}

func (r Reminder) Value() (driver.Value, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (r *Reminder) Scan(src any) error {
	if src == nil {
		*r = DefaultReminder()
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("reminder: unsupported type %T", src)
	}
	if len(raw) == 0 {
		*r = DefaultReminder()
		return nil
	}
	return json.Unmarshal(raw, r)
}

// Tempo is a reviewable garden item owned by one user (auth uid).
type Tempo struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	UserID       string    `gorm:"size:36;not null;index:idx_tempos_user" json:"userId"`
	Name         string    `gorm:"size:255;not null" json:"name"`
	Content      string    `gorm:"type:mediumtext" json:"content"`
	Confidence   int       `gorm:"not null;default:3" json:"confidence"`
	Due          time.Time `gorm:"not null;index:idx_tempos_due" json:"due"`
	Ease         float64   `gorm:"not null;default:2.3" json:"ease"`
	IntervalDays float64   `gorm:"not null;default:0" json:"intervalDays"`
	Reminder     Reminder  `gorm:"type:json;not null" json:"reminder"`
	Scale        float64   `gorm:"not null;default:1" json:"scale"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`

	Examples []Example  `gorm:"foreignKey:TempoID;constraint:OnDelete:CASCADE" json:"-"`
	Media    []Media    `gorm:"foreignKey:TempoID;constraint:OnDelete:CASCADE" json:"-"`
	Tags     []TempoTag `gorm:"foreignKey:TempoID;constraint:OnDelete:CASCADE" json:"-"`
}

func (Tempo) TableName() string { return "tempos" }

// ExampleDTO is the public example shape.
type ExampleDTO struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// MediaDTO is the public media shape.
type MediaDTO struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

// TempoDetail is GET /tempos/:id.
type TempoDetail struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Content      string       `json:"content"`
	Examples     []ExampleDTO `json:"examples"`
	Tags         []string     `json:"tags"`
	Media        []MediaDTO   `json:"media"`
	Confidence   int          `json:"confidence"`
	Due          string       `json:"due"`
	Ease         float64      `json:"ease"`
	IntervalDays float64      `json:"intervalDays"`
	Reminder     Reminder     `json:"reminder"`
	Scale        float64      `json:"scale"`
}

// Bubble scale + name limits. Must match mdg-web-ui.
const (
	MinBubbleScale = 0.35
	MaxBubbleScale = 3
	// MaxEntityName is the max runes for tempo names (bubble labels).
	MaxEntityName = 48
)

// ClampScale returns a valid bubble scale (default 1).
func ClampScale(s float64) float64 {
	if s <= 0 || s != s { // NaN / non-positive → default
		return 1
	}
	if s < MinBubbleScale {
		return MinBubbleScale
	}
	if s > MaxBubbleScale {
		return MaxBubbleScale
	}
	return s
}

// NormalizeName trims whitespace. Empty if blank.
func NormalizeName(name string) string {
	return strings.TrimSpace(name)
}

// ValidName reports whether name is non-empty and within MaxEntityName runes.
func ValidName(name string) bool {
	n := NormalizeName(name)
	if n == "" {
		return false
	}
	return utf8.RuneCountInString(n) <= MaxEntityName
}

// ScaleForName picks a bubble scale that fits the label length.
// Short names sit under 1×; longer names grow toward ~2× (clamped).
func ScaleForName(name string) float64 {
	n := utf8.RuneCountInString(NormalizeName(name))
	if n <= 0 {
		return 1
	}
	var s float64
	switch {
	case n <= 8:
		s = 0.7 + float64(n-1)*(0.25/7)
	case n <= 20:
		s = 0.95 + float64(n-8)*(0.35/12)
	case n <= 36:
		s = 1.3 + float64(n-20)*(0.4/16)
	default:
		s = 1.7 + float64(n-36)*(0.5/12)
	}
	return ClampScale(s)
}

// ReviewQueueItem is a lightweight entry in GET /review (no content payload).
type ReviewQueueItem struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Tags       []string `json:"tags"`
	Confidence int      `json:"confidence"`
	Due        string   `json:"due"`
}

// ReviewTempo is GET /review/tempos/:id — content shaped for the review card.
type ReviewTempo struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Content  string       `json:"content"`
	Examples []ExampleDTO `json:"examples"`
	Tags     []string     `json:"tags"`
	Media    []MediaDTO   `json:"media"`
}

// ScheduleResult is the POST /tempos/:id/reviews response.
type ScheduleResult struct {
	ID           string  `json:"id"`
	Confidence   int     `json:"confidence"`
	Ease         float64 `json:"ease"`
	IntervalDays float64 `json:"intervalDays"`
	Due          string  `json:"due"`
}

func FormatDue(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
