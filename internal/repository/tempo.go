package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/models"
)

// TempoRepository owns tempos (reviewable garden items).
type TempoRepository struct {
	db *gorm.DB
}

func NewTempoRepository(db *gorm.DB) *TempoRepository {
	return &TempoRepository{db: db}
}

func (r *TempoRepository) FindByID(ctx context.Context, userID string, id string) (*models.Tempo, error) {
	var t models.Tempo
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND id = ?", userID, id).
		First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &t, nil
}

func (r *TempoRepository) Save(ctx context.Context, t *models.Tempo) error {
	return r.db.WithContext(ctx).Save(t).Error
}

func (r *TempoRepository) Create(ctx context.Context, t *models.Tempo) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *TempoRepository) Delete(ctx context.Context, userID string, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var t models.Tempo
		if err := tx.Where("user_id = ? AND id = ?", userID, id).First(&t).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}

		res := tx.Where("user_id = ? AND id = ?", userID, id).Delete(&models.Tempo{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// MapListOpts filters and pages tempo rows for GET /map.
type MapListOpts struct {
	Q       string
	Tag     string
	OnlyDue bool
	Now     time.Time
	Limit   int
	Offset  int
}

// ListMapPage returns lightweight tempo rows for the chart (no content/relations).
// Caller must pass a valid Limit (1–100) and Offset (>= 0).
// Order: due first, then lowest confidence, then earliest due, then name.
// mapPageQuery returns a new session with the map filters applied.
// Call it once per goroutine. A GORM chain mutates its statement, so two
// queries must not share one chain.
func (r *TempoRepository) mapPageQuery(ctx context.Context, userID string, opts MapListOpts) *gorm.DB {
	base := r.db.WithContext(ctx).Session(&gorm.Session{}).
		Model(&models.Tempo{}).
		Where("tempos.user_id = ?", userID)
	if opts.OnlyDue {
		base = base.Where("tempos.due <= ?", opts.Now)
	}
	if q := strings.TrimSpace(opts.Q); q != "" {
		base = base.Where("LOWER(tempos.name) LIKE ?", "%"+strings.ToLower(q)+"%")
	}
	if tag := strings.TrimSpace(opts.Tag); tag != "" {
		base = base.Where(
			"EXISTS (SELECT 1 FROM tempo_tags WHERE tempo_tags.tempo_id = tempos.id AND LOWER(tempo_tags.tag) = ?)",
			strings.ToLower(tag),
		)
	}
	return base
}

func (r *TempoRepository) ListMapPage(ctx context.Context, userID string, opts MapListOpts) ([]models.Tempo, int64, error) {
	g, ctx := errgroup.WithContext(ctx)
	var total int64
	var tempos []models.Tempo
	g.Go(func() error {
		return r.mapPageQuery(ctx, userID, opts).Count(&total).Error
	})
	g.Go(func() error {
		return r.mapPageQuery(ctx, userID, opts).
			Select("tempos.id", "tempos.name", "tempos.confidence", "tempos.due", "tempos.scale").
			Order(clause.Expr{SQL: "CASE WHEN tempos.due <= ? THEN 0 ELSE 1 END ASC", Vars: []any{opts.Now}}).
			Order("tempos.confidence ASC").
			Order("tempos.due ASC").
			Order("tempos.name ASC").
			Limit(opts.Limit).
			Offset(opts.Offset).
			Find(&tempos).Error
	})
	if err := g.Wait(); err != nil {
		return nil, 0, err
	}
	return tempos, total, nil
}

// ReminderTarget is one tempo and the Telegram chat that should receive it.
type ReminderTarget struct {
	ID     string
	UserID string
	Name   string
	ChatID string
}

// NextToRemind returns one tempo whose owner has a Telegram chat id.
// Never-reminded tempos come first, then the one reminded longest ago.
func (r *TempoRepository) NextToRemind(ctx context.Context) (*ReminderTarget, error) {
	var rows []ReminderTarget
	err := r.db.WithContext(ctx).
		Table("tempos").
		Select("tempos.id, tempos.user_id, tempos.name, user_telegram.chat_id").
		Joins("JOIN user_telegram ON user_telegram.user_id = tempos.user_id AND user_telegram.chat_id <> ''").
		Joins(`LEFT JOIN (
			SELECT tempo_id, MAX(created_at) AS last_at
			FROM activity_log
			WHERE channel = ? AND status = ?
			GROUP BY tempo_id
		) reminded ON reminded.tempo_id = tempos.id`, models.ChannelTelegram, models.StatusReminded).
		Order(clause.Expr{SQL: "reminded.last_at IS NULL DESC"}).
		Order(clause.Expr{SQL: "reminded.last_at ASC"}).
		Order("tempos.due ASC").
		Order("tempos.id ASC").
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	return &rows[0], nil
}

// CountDue returns how many tempos are due for the user.
func (r *TempoRepository) CountDue(ctx context.Context, userID string, now time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Tempo{}).
		Where("user_id = ? AND due <= ?", userID, now).
		Count(&n).Error
	return n, err
}

// ListDueTempos returns tempos whose due time is <= now.
func (r *TempoRepository) ListDueTempos(ctx context.Context, userID string, now time.Time) ([]models.Tempo, error) {
	var tempos []models.Tempo
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND due <= ?", userID, now).
		Order("due ASC").
		Find(&tempos).Error
	return tempos, err
}

// ListLowestConfidenceTempos returns up to limit tempos ordered by confidence ASC, due ASC.
func (r *TempoRepository) ListLowestConfidenceTempos(ctx context.Context, userID string, limit int) ([]models.Tempo, error) {
	var tempos []models.Tempo
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("confidence ASC, due ASC").
		Limit(limit).
		Find(&tempos).Error
	return tempos, err
}

// LoadRelations fills examples, media, and tags for a tempo.
func (r *TempoRepository) LoadRelations(ctx context.Context, t *models.Tempo) error {
	if err := r.db.WithContext(ctx).
		Where("tempo_id = ?", t.ID).
		Order("sort_order ASC, created_at ASC").
		Find(&t.Examples).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).
		Where("tempo_id = ?", t.ID).
		Order("sort_order ASC, created_at ASC").
		Find(&t.Media).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).
		Where("tempo_id = ?", t.ID).
		Order("tag ASC").
		Find(&t.Tags).Error; err != nil {
		return err
	}
	return nil
}
