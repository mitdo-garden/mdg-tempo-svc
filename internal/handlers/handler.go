package handlers

import (
	"log/slog"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/config"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/repository"
)

// Handler serves Review, Tempo, and Map APIs.
type Handler struct {
	cfg      *config.Config
	tempos   *repository.TempoRepository
	examples *repository.ExampleRepository
	media    *repository.MediaRepository
	tags     *repository.TagRepository
	history  *repository.HistoryRepository
	telegram *repository.TelegramRepository
	logger   *slog.Logger
}

func New(
	cfg *config.Config,
	tempos *repository.TempoRepository,
	examples *repository.ExampleRepository,
	media *repository.MediaRepository,
	tags *repository.TagRepository,
	history *repository.HistoryRepository,
	telegram *repository.TelegramRepository,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		cfg:      cfg,
		tempos:   tempos,
		examples: examples,
		media:    media,
		tags:     tags,
		history:  history,
		telegram: telegram,
		logger:   logger,
	}
}
