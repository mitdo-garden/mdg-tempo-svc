package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/models"
)

type ExampleRepository struct {
	db *gorm.DB
}

func NewExampleRepository(db *gorm.DB) *ExampleRepository {
	return &ExampleRepository{db: db}
}

func (r *ExampleRepository) Create(ctx context.Context, e *models.Example) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *ExampleRepository) NextSortOrder(ctx context.Context, tempoID string) (int, error) {
	var max *int
	err := r.db.WithContext(ctx).Model(&models.Example{}).
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

func (r *ExampleRepository) Delete(ctx context.Context, tempoID, exampleID string) error {
	res := r.db.WithContext(ctx).
		Where("tempo_id = ? AND id = ?", tempoID, exampleID).
		Delete(&models.Example{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ExampleRepository) Find(ctx context.Context, tempoID, exampleID string) (*models.Example, error) {
	var e models.Example
	err := r.db.WithContext(ctx).
		Where("tempo_id = ? AND id = ?", tempoID, exampleID).
		First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ExampleRepository) UpdateText(ctx context.Context, tempoID, exampleID, text string) (*models.Example, error) {
	e, err := r.Find(ctx, tempoID, exampleID)
	if err != nil {
		return nil, err
	}
	e.Text = text
	if err := r.db.WithContext(ctx).Model(e).Update("text", text).Error; err != nil {
		return nil, err
	}
	return e, nil
}
