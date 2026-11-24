package repository

import (
	"context"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/models"
)

type TagRepository struct {
	db *gorm.DB
}

func NewTagRepository(db *gorm.DB) *TagRepository {
	return &TagRepository{db: db}
}

// ReplaceTags atomically replaces all tags on a tempo. Empty list stays empty.
// Returns the normalized tags that were stored (sorted).
func (r *TagRepository) ReplaceTags(ctx context.Context, tempoID string, tags []string) ([]string, error) {
	normalized := normalizeTags(tags)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tempo_id = ?", tempoID).Delete(&models.TempoTag{}).Error; err != nil {
			return err
		}
		for _, tag := range normalized {
			if err := tx.Create(&models.TempoTag{TempoID: tempoID, Tag: tag}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return normalized, err
}

// TagsByTempoIDs returns tag names keyed by tempo id.
func (r *TagRepository) TagsByTempoIDs(ctx context.Context, tempoIDs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(tempoIDs))
	if len(tempoIDs) == 0 {
		return out, nil
	}
	var rows []models.TempoTag
	if err := r.db.WithContext(ctx).
		Where("tempo_id IN ?", tempoIDs).
		Order("tag ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.TempoID] = append(out[row.TempoID], row.Tag)
	}
	return out, nil
}

func (r *TagRepository) Search(ctx context.Context, userID string, q string, limit int) ([]models.TagCount, error) {
	if limit <= 0 {
		limit = 20
	}
	q = strings.Join(strings.Fields(strings.ToLower(q)), " ")

	type row struct {
		Name  string
		Count int64
	}
	var rows []row
	query := r.db.WithContext(ctx).
		Table("tempo_tags").
		Select("tempo_tags.tag AS name, COUNT(*) AS count").
		Joins("JOIN tempos ON tempos.id = tempo_tags.tempo_id").
		Where("tempos.user_id = ?", userID).
		Group("tempo_tags.tag").
		Order("count DESC, name ASC").
		Limit(limit)

	if q != "" {
		query = query.Where("tempo_tags.tag LIKE ?", "%"+q+"%")
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	if q != "" {
		var exact []row
		err := r.db.WithContext(ctx).
			Table("tempo_tags").
			Select("tempo_tags.tag AS name, COUNT(*) AS count").
			Joins("JOIN tempos ON tempos.id = tempo_tags.tempo_id").
			Where("tempos.user_id = ?", userID).
			Where("LOWER(tempo_tags.tag) = ?", q).
			Group("tempo_tags.tag").
			Scan(&exact).Error
		if err != nil {
			return nil, err
		}
		if len(exact) > 0 {
			rows = append(exact, rows...)
		}
	}

	out := make([]models.TagCount, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		key := strings.Join(strings.Fields(strings.ToLower(row.Name)), " ")
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, models.TagCount{Name: row.Name, Count: row.Count})
	}
	return out, nil
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.Join(strings.Fields(strings.ToLower(t)), " ")
		if t == "" {
			continue
		}
		if len(t) > 64 {
			t = t[:64]
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
