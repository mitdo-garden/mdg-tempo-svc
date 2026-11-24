package handlers

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/httperr"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/middleware"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/models"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/repository"
)

func userID(c fiber.Ctx) (string, error) {
	id, ok := middleware.UserID(c)
	if !ok {
		return "", httperr.Write(c, fiber.StatusUnauthorized, "unauthenticated", "Sign in required.")
	}
	return id, nil
}

func (h *Handler) log(c fiber.Ctx) *slog.Logger {
	return middleware.Logger(c, h.logger)
}

// reqAttrs attaches stable request fields at the top level.
// Caller-supplied key/value pairs go under contextData for a single nested blob.
func reqAttrs(c fiber.Ctx, op string, attrs []any) []any {
	args := make([]any, 0, 8)
	args = append(args, "method", c.Method(), "path", c.Path())
	if op != "" {
		args = append(args, "op", op)
	}
	if uid, ok := middleware.UserID(c); ok {
		args = append(args, "user_id", uid)
	}
	if len(attrs) > 0 {
		args = append(args, slog.Group("contextData", attrs...))
	}
	return args
}

func (h *Handler) emit(c fiber.Ctx, level slog.Level, msg, op string, attrs ...any) {
	if msg == "" {
		msg = "api"
	}
	h.log(c).Log(c.Context(), level, msg, reqAttrs(c, op, attrs)...)
}

// logOp records the start of an API operation.
// msg is a human-readable note (may be empty → "api_start").
// Example: h.logOp(c, "get_map", "loading garden", "q", q)
func (h *Handler) logOp(c fiber.Ctx, op, msg string, attrs ...any) {
	if msg == "" {
		msg = "api_start"
	}
	h.emit(c, slog.LevelInfo, msg, op, attrs...)
}

// logOK records a successful API operation result.
// msg is a human-readable note (may be empty → "api_ok").
func (h *Handler) logOK(c fiber.Ctx, op, msg string, attrs ...any) {
	if msg == "" {
		msg = "api_ok"
	}
	h.emit(c, slog.LevelInfo, msg, op, attrs...)
}

func mapRepoErr(c fiber.Ctx, err error, notFoundMsg string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound), errors.Is(err, repository.ErrNotTempo):
		return httperr.Write(c, fiber.StatusNotFound, "not_found", notFoundMsg)
	case errors.Is(err, repository.ErrConflict):
		return httperr.Write(c, fiber.StatusConflict, "conflict", "Already exists.")
	default:
		return err
	}
}

func exampleDTOs(examples []models.Example) []models.ExampleDTO {
	out := make([]models.ExampleDTO, 0, len(examples))
	for _, e := range examples {
		out = append(out, e.ToDTO())
	}
	return out
}

func mediaDTOs(media []models.Media) []models.MediaDTO {
	out := make([]models.MediaDTO, 0, len(media))
	for _, m := range media {
		out = append(out, m.ToDTO())
	}
	return out
}

func tagNames(tags []models.TempoTag) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, t.Tag)
	}
	return out
}

func logInternal(logger *slog.Logger, c fiber.Ctx, msg string, err error) {
	attrs := []any{
		"method", c.Method(),
		"path", c.Path(),
		"err", err,
	}
	if uid, ok := middleware.UserID(c); ok {
		attrs = append(attrs, "user_id", uid)
	}
	middleware.Logger(c, logger).Error(msg, attrs...)
}
