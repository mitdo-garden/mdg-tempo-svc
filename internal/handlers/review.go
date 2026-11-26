package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/httperr"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/models"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/reqvalid"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/schedule"
)

type reviewQuery struct {
	Seed string `query:"seed"`
}

// GetReview — GET /review
// Returns a lightweight queue (ids + labels). Full card content is loaded
// on demand via GET /review/tempos/:id.
func (h *Handler) GetReview(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}

	var query reviewQuery
	if err := reqvalid.Query(c, &query); err != nil {
		return err
	}

	seed := strings.TrimSpace(query.Seed)
	if seed == "" {
		seed = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	h.logOp(c, "get_review", "building review queue", "seed", seed)
	now := time.Now().UTC()

	due, err := h.tempos.ListDueTempos(c.Context(), uid, now, seed)
	if err != nil {
		logInternal(h.logger, c, "list due tempos failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}

	source := "due"
	tempos := due
	if len(tempos) == 0 {
		source = "extra"
		tempos, err = h.tempos.ListLowestConfidenceTempos(c.Context(), uid, 5)
		if err != nil {
			logInternal(h.logger, c, "list lowest confidence tempos failed", err)
			return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
		}
	}

	ids := make([]string, 0, len(tempos))
	for _, t := range tempos {
		ids = append(ids, t.ID)
	}
	tagMap, err := h.tags.TagsByTempoIDs(c.Context(), ids)
	if err != nil {
		logInternal(h.logger, c, "list review queue tags failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}

	items := make([]models.ReviewQueueItem, 0, len(tempos))
	for _, t := range tempos {
		tags := tagMap[t.ID]
		if tags == nil {
			tags = []string{}
		}
		items = append(items, models.ReviewQueueItem{
			ID:         t.ID,
			Name:       t.Name,
			Tags:       tags,
			Confidence: t.Confidence,
			Due:        models.FormatDue(t.Due),
		})
	}

	h.logOK(c, "get_review", "review queue ready", "source", source, "tempo_count", len(items))
	return c.JSON(fiber.Map{
		"source": source,
		"items":  items,
		"seed":   seed,
	})
}

// GetReviewTempo — GET /review/tempos/:id
// Card-shaped payload for one tempo in the review queue.
func (h *Handler) GetReviewTempo(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	h.logOp(c, "get_review_tempo", "", "tempo_id", id)

	t, err := h.tempos.FindByID(c.Context(), uid, id)
	if err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}
	if err := h.tempos.LoadRelations(c.Context(), t); err != nil {
		logInternal(h.logger, c, "load review tempo relations failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}

	h.logOK(c, "get_review_tempo", "", "tempo_id", id)
	return c.JSON(models.ReviewTempo{
		ID:       t.ID,
		Name:     t.Name,
		Content:  t.Content,
		Examples: exampleDTOs(t.Examples),
		Tags:     tagNames(t.Tags),
		Media:    mediaDTOs(t.Media),
	})
}

type reviewBody struct {
	Rating int `json:"rating" validate:"min=1,max=5"`
}

// PostReview — POST /tempos/:id/reviews
func (h *Handler) PostReview(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	h.logOp(c, "post_review", "applying review rating", "tempo_id", id)

	var body reviewBody
	if err := reqvalid.Body(c, &body); err != nil {
		return err
	}

	t, err := h.tempos.FindByID(c.Context(), uid, id)
	if err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}

	now := time.Now().UTC()
	next := schedule.Apply(t.Ease, t.IntervalDays, body.Rating, now)
	t.Confidence = next.Confidence
	t.Ease = next.Ease
	t.IntervalDays = next.IntervalDays
	t.Due = next.Due.UTC()

	if err := h.tempos.Save(c.Context(), t); err != nil {
		logInternal(h.logger, c, "save reviewed tempo failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	if err := h.history.AppendReview(c.Context(), uid, t.ID, body.Rating, now); err != nil {
		logInternal(h.logger, c, "append review history failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}

	h.logOK(c, "post_review", "schedule updated after review", "tempo_id", t.ID, "rating", body.Rating, "confidence", t.Confidence, "interval_days", t.IntervalDays)
	return c.JSON(models.ScheduleResult{
		ID:           t.ID,
		Confidence:   t.Confidence,
		Ease:         t.Ease,
		IntervalDays: t.IntervalDays,
		Due:          models.FormatDue(t.Due),
	})
}

type exampleBody struct {
	Text string `json:"text" validate:"required,min=1"`
}

func (b *exampleBody) Normalize() {
	b.Text = strings.TrimSpace(b.Text)
}

// PostExample — POST /tempos/:id/examples
func (h *Handler) PostExample(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	h.logOp(c, "post_example", "", "tempo_id", id)

	var body exampleBody
	if err := reqvalid.Body(c, &body); err != nil {
		return err
	}

	if _, err := h.tempos.FindByID(c.Context(), uid, id); err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}

	order, err := h.examples.NextSortOrder(c.Context(), id)
	if err != nil {
		logInternal(h.logger, c, "example sort order failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}

	e := &models.Example{
		ID:        uuid.NewString(),
		TempoID:   id,
		Text:      body.Text,
		SortOrder: order,
	}
	if err := h.examples.Create(c.Context(), e); err != nil {
		logInternal(h.logger, c, "create example failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	h.logOK(c, "post_example", "", "tempo_id", id, "example_id", e.ID)
	return c.Status(fiber.StatusCreated).JSON(e.ToDTO())
}

// GenerateExample — POST /tempos/:id/examples/generate
func (h *Handler) GenerateExample(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	h.logOp(c, "generate_example", "", "tempo_id", id)

	t, err := h.tempos.FindByID(c.Context(), uid, id)
	if err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}
	if err := h.tempos.LoadRelations(c.Context(), t); err != nil {
		logInternal(h.logger, c, "load tempo for generate failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}

	pool := []string{
		fmt.Sprintf(`Explain "%s" to a 10-year-old in 2 sentences.`, t.Name),
		fmt.Sprintf(`Find a situation at work where "%s" shows up.`, t.Name),
		fmt.Sprintf(`What is a common mistake when applying "%s"? Give a wrong example and fix it.`, t.Name),
		fmt.Sprintf(`Compare "%s" with a concept you already know well.`, t.Name),
	}
	text := pool[len(t.Examples)%len(pool)]

	order, err := h.examples.NextSortOrder(c.Context(), id)
	if err != nil {
		logInternal(h.logger, c, "example sort order failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}

	e := &models.Example{
		ID:        uuid.NewString(),
		TempoID:   id,
		Text:      text,
		SortOrder: order,
	}
	if err := h.examples.Create(c.Context(), e); err != nil {
		logInternal(h.logger, c, "create generated example failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	h.logOK(c, "generate_example", "", "tempo_id", id, "example_id", e.ID)
	return c.Status(fiber.StatusCreated).JSON(e.ToDTO())
}

// PatchExample — PATCH /tempos/:id/examples/:exampleId
func (h *Handler) PatchExample(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	exampleID := c.Params("exampleId")
	h.logOp(c, "patch_example", "", "tempo_id", id, "example_id", exampleID)

	var body exampleBody
	if err := reqvalid.Body(c, &body); err != nil {
		return err
	}

	if _, err := h.tempos.FindByID(c.Context(), uid, id); err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}
	e, err := h.examples.UpdateText(c.Context(), id, exampleID, body.Text)
	if err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Example %s was not found.", exampleID))
	}
	h.logOK(c, "patch_example", "", "tempo_id", id, "example_id", exampleID)
	return c.JSON(e.ToDTO())
}

// DeleteExample — DELETE /tempos/:id/examples/:exampleId
func (h *Handler) DeleteExample(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	exampleID := c.Params("exampleId")
	h.logOp(c, "delete_example", "", "tempo_id", id, "example_id", exampleID)

	if _, err := h.tempos.FindByID(c.Context(), uid, id); err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}
	if err := h.examples.Delete(c.Context(), id, exampleID); err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Example %s was not found.", exampleID))
	}
	h.logOK(c, "delete_example", "", "tempo_id", id, "example_id", exampleID)
	return c.SendStatus(fiber.StatusNoContent)
}
