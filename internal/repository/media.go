package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/models"
)

type MediaRepository struct {
	db *gorm.DB
}

func NewMediaRepository(db *gorm.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

func (r *MediaRepository) Create(ctx context.Context, m *models.Media) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *MediaRepository) NextSortOrder(ctx context.Context, tempoID string) (int, error) {
	var max *int
	err := r.db.WithContext(ctx).Model(&models.Media{}).
		Select("MAX(sort_order)").
		Where("tempo_id = ?", tempoID).
		Scan(&max).Error
	if err != nil {
		return 0, err
	}
	if max == nil {
		return 0, nil
	}
	return *max + 1, nil
}

func (r *MediaRepository) Delete(ctx context.Context, tempoID, mediaID string) error {
	res := r.db.WithContext(ctx).
		Where("tempo_id = ? AND id = ?", tempoID, mediaID).
		Delete(&models.Media{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MediaRepository) Find(ctx context.Context, tempoID, mediaID string) (*models.Media, error) {
	var m models.Media
	err := r.db.WithContext(ctx).
		Where("tempo_id = ? AND id = ?", tempoID, mediaID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}
