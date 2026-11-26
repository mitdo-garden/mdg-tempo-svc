package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/sync/errgroup"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/httperr"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/models"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/repository"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/reqvalid"
)

const defaultMapLimit = 40

// mapQuery is GET /map query params (Fiber query tags + validator annotations).
type mapQuery struct {
	Q      string `query:"q"`
	Due    string `query:"due" validate:"omitempty,oneof=true false 1 0"`
	Tag    string `query:"tag"`
	Seed   string `query:"seed"`
	Limit  *int   `query:"limit" validate:"omitempty,min=1,max=100"`
	Offset *int   `query:"offset" validate:"omitempty,min=0"`
}

func (q mapQuery) limitOrDefault() int {
	if q.Limit == nil {
		return defaultMapLimit
	}
	return *q.Limit
}

func (q mapQuery) offsetOrDefault() int {
	if q.Offset == nil {
		return 0
	}
	return *q.Offset
}

func (q mapQuery) onlyDue() bool {
	return q.Due == "true" || q.Due == "1"
}

// GetMap — GET /map?q=&due=true&tag=&limit=&offset=
// Returns list stubs only (no content). Detail is GET /tempos/:id.
func (h *Handler) GetMap(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}

	var query mapQuery
	if err := reqvalid.Query(c, &query); err != nil {
		return err
	}

	q := strings.TrimSpace(query.Q)
	tagFilter := strings.TrimSpace(query.Tag)
	onlyDue := query.onlyDue()
	limit := query.limitOrDefault()
	offset := query.offsetOrDefault()
	seed := strings.TrimSpace(query.Seed)
	if seed == "" {
		seed = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	h.logOp(c, "get_map", "loading garden map", "q", q, "tag", tagFilter, "due", onlyDue, "seed", seed, "limit", limit, "offset", offset)

	now := time.Now().UTC()
	g, ctx := errgroup.WithContext(c.Context())
	var tempos []models.Tempo
	var total int64
	var dueCount int64
	g.Go(func() error {
		var err error
		tempos, total, err = h.tempos.ListMapPage(ctx, uid, repository.MapListOpts{
			Q:       q,
			Tag:     tagFilter,
			OnlyDue: onlyDue,
			Now:     now,
			Seed:    seed,
			Limit:   limit,
			Offset:  offset,
		})
		if err != nil {
			return fmt.Errorf("list map tempos: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		var err error
		dueCount, err = h.tempos.CountDue(ctx, uid, now)
		if err != nil {
			return fmt.Errorf("count due tempos: %w", err)
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		logInternal(h.logger, c, "load garden map failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}

	ids := make([]string, 0, len(tempos))
	for _, t := range tempos {
		ids = append(ids, t.ID)
	}
	tagMap, err := h.tags.TagsByTempoIDs(c.Context(), ids)
	if err != nil {
		logInternal(h.logger, c, "list map tags failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}

	outTempos := make([]fiber.Map, 0, len(tempos))
	for _, t := range tempos {
		tags := tagMap[t.ID]
		if tags == nil {
			tags = []string{}
		}
		outTempos = append(outTempos, fiber.Map{
			"id":         t.ID,
			"name":       t.Name,
			"confidence": t.Confidence,
			"due":        models.FormatDue(t.Due),
			"tags":       tags,
			"scale":      models.ClampScale(t.Scale),
		})
	}

	hasMore := int64(offset+len(outTempos)) < total
	h.logOK(c, "get_map", "garden map loaded",
		"tempo_count", len(outTempos),
		"total", total,
		"has_more", hasMore,
	)
	return c.JSON(fiber.Map{
		"tempos":   outTempos,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"hasMore":  hasMore,
		"dueCount": dueCount,
		"seed":     seed,
	})
}
