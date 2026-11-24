package handlers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/httperr"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/models"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/repository"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/reqvalid"
)

type createTempoBody struct {
	ID           string   `json:"id"`
	Name         string   `json:"name" validate:"required,entityname"`
	Content      string   `json:"content"`
	Confidence   *int     `json:"confidence" validate:"omitempty,min=1,max=5"`
	Due          *string  `json:"due" validate:"omitempty,rfc3339"`
	Ease         *float64 `json:"ease"`
	IntervalDays *float64 `json:"intervalDays"`
}

func (b *createTempoBody) Normalize() {
	b.Name = models.NormalizeName(b.Name)
	if b.Due != nil {
		trimmed := strings.TrimSpace(*b.Due)
		b.Due = &trimmed
	}
}

// PostTempo — POST /tempos (create tempo; optional client id for sync)
func (h *Handler) PostTempo(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}

	var body createTempoBody
	if err := reqvalid.Body(c, &body); err != nil {
		return err
	}
	h.logOp(c, "post_tempo", "creating tempo", "name", body.Name)

	id := strings.TrimSpace(body.ID)
	if id != "" {
		if _, err := h.tempos.FindByID(c.Context(), uid, id); err == nil {
			return httperr.Write(c, fiber.StatusConflict, "conflict", "A tempo with that id already exists.")
		} else if !errors.Is(err, repository.ErrNotFound) {
			logInternal(h.logger, c, "check tempo id failed", err)
			return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
		}
	} else {
		id = uuid.NewString()
	}

	now := time.Now().UTC()
	t := &models.Tempo{
		ID:           id,
		UserID:       uid,
		Name:         body.Name,
		Content:      body.Content,
		Confidence:   1, // matches UI new-tempo default
		Due:          now,
		Ease:         2.3,
		IntervalDays: 0,
		Reminder:     models.DefaultReminder(),
		Scale:        models.ScaleForName(body.Name),
	}
	if body.Confidence != nil {
		t.Confidence = *body.Confidence
	}
	if body.Ease != nil {
		t.Ease = *body.Ease
	}
	if body.IntervalDays != nil {
		t.IntervalDays = *body.IntervalDays
	}
	if body.Due != nil && *body.Due != "" {
		if due, err := time.Parse(time.RFC3339, *body.Due); err == nil {
			t.Due = due.UTC()
		}
	}

	if err := h.tempos.Create(c.Context(), t); err != nil {
		logInternal(h.logger, c, "create tempo failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}

	detail, err := h.buildTempoDetail(c, uid, t)
	if err != nil {
		logInternal(h.logger, c, "build created tempo failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	h.logOK(c, "post_tempo", "created", "tempo_id", t.ID, "name", t.Name)
	return c.Status(fiber.StatusCreated).JSON(detail)
}

// GetTempo — GET /tempos/:id
func (h *Handler) GetTempo(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	h.logOp(c, "get_tempo", "", "tempo_id", id)

	t, err := h.tempos.FindByID(c.Context(), uid, id)
	if err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}

	detail, err := h.buildTempoDetail(c, uid, t)
	if err != nil {
		logInternal(h.logger, c, "build tempo detail failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	h.logOK(c, "get_tempo", "", "tempo_id", id, "name", t.Name)
	return c.JSON(detail)
}

type patchTempoBody struct {
	Name    *string  `json:"name" validate:"omitempty,entityname"`
	Content *string  `json:"content"`
	Scale   *float64 `json:"scale" validate:"omitempty,min=0.35,max=3"`
}

func (b *patchTempoBody) Normalize() {
	if b.Name != nil {
		n := models.NormalizeName(*b.Name)
		b.Name = &n
	}
}

// PatchTempo — PATCH /tempos/:id
func (h *Handler) PatchTempo(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	h.logOp(c, "patch_tempo", "", "tempo_id", id)

	var body patchTempoBody
	if err := reqvalid.Body(c, &body); err != nil {
		return err
	}

	t, err := h.tempos.FindByID(c.Context(), uid, id)
	if err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}

	nameTouched := false
	if body.Name != nil {
		t.Name = *body.Name
		nameTouched = true
	}
	if body.Content != nil {
		t.Content = *body.Content
	}
	if body.Scale != nil {
		t.Scale = *body.Scale
	} else if nameTouched {
		// Renaming auto-sizes the bubble unless the client also sent scale.
		t.Scale = models.ScaleForName(t.Name)
	}

	if err := h.tempos.Save(c.Context(), t); err != nil {
		logInternal(h.logger, c, "patch tempo failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}

	detail, err := h.buildTempoDetail(c, uid, t)
	if err != nil {
		logInternal(h.logger, c, "build tempo detail after patch failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	h.logOK(c, "patch_tempo", "", "tempo_id", t.ID, "name", t.Name)
	return c.JSON(detail)
}

// DeleteTempo — DELETE /tempos/:id
func (h *Handler) DeleteTempo(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	h.logOp(c, "delete_tempo", "deleting tempo", "tempo_id", id)
	if err := h.tempos.Delete(c.Context(), uid, id); err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}
	h.logOK(c, "delete_tempo", "deleted", "tempo_id", id)
	return c.SendStatus(fiber.StatusNoContent)
}

type tagsBody struct {
	Tags []string `json:"tags" validate:"dive,min=1,max=64"`
}

func (b *tagsBody) Normalize() {
	if b.Tags == nil {
		b.Tags = []string{}
		return
	}
	for i, t := range b.Tags {
		b.Tags[i] = strings.TrimSpace(strings.ToLower(t))
	}
}

// PutTags — PUT /tempos/:id/tags
func (h *Handler) PutTags(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	h.logOp(c, "put_tags", "", "tempo_id", id)

	var body tagsBody
	if err := reqvalid.Body(c, &body); err != nil {
		return err
	}

	if _, err := h.tempos.FindByID(c.Context(), uid, id); err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}
	tags, err := h.tags.ReplaceTags(c.Context(), id, body.Tags)
	if err != nil {
		logInternal(h.logger, c, "replace tags failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	h.logOK(c, "put_tags", "", "tempo_id", id, "tag_count", len(tags))
	return c.JSON(fiber.Map{"tags": tags})
}

type tagsQuery struct {
	Q string `query:"q" validate:"omitempty,max=64"`
}

func (q *tagsQuery) Normalize() {
	q.Q = strings.TrimSpace(q.Q)
}

// GetTags — GET /tags?q=
func (h *Handler) GetTags(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	var query tagsQuery
	if err := reqvalid.Query(c, &query); err != nil {
		return err
	}
	h.logOp(c, "get_tags", "", "q", query.Q)
	tags, err := h.tags.Search(c.Context(), uid, query.Q, 30)
	if err != nil {
		logInternal(h.logger, c, "search tags failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	if tags == nil {
		tags = []models.TagCount{}
	}
	h.logOK(c, "get_tags", "", "count", len(tags))
	return c.JSON(fiber.Map{"tags": tags})
}

type mediaBody struct {
	Kind  string `json:"kind" validate:"required,mediakind"`
	URL   string `json:"url" validate:"required"`
	Title string `json:"title" validate:"omitempty,max=255"`
}

func (b *mediaBody) Normalize() {
	b.Kind = strings.TrimSpace(b.Kind)
	b.URL = strings.TrimSpace(b.URL)
	b.Title = strings.TrimSpace(b.Title)
}

// PostMedia — POST /tempos/:id/media (JSON or multipart)
func (h *Handler) PostMedia(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	h.logOp(c, "post_media", "", "tempo_id", id)

	if _, err := h.tempos.FindByID(c.Context(), uid, id); err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}

	kind := strings.TrimSpace(c.FormValue("kind"))
	title := strings.TrimSpace(c.FormValue("title"))
	url := strings.TrimSpace(c.FormValue("url"))
	if kind == "" {
		var parsed mediaBody
		if err := c.Bind().Body(&parsed); err == nil {
			parsed.Normalize()
			kind, title, url = parsed.Kind, parsed.Title, parsed.URL
		}
	}

	if file, err := c.FormFile("file"); err == nil && file != nil {
		meta := struct {
			Kind  string `validate:"required,mediakind"`
			Title string `validate:"omitempty,max=255"`
		}{Kind: kind, Title: title}
		if err := reqvalid.Struct(&meta); err != nil {
			return httperr.Write(c, fiber.StatusBadRequest, "bad_request", reqvalid.Message(err))
		}
		kind = strings.TrimSpace(meta.Kind)
		if err := os.MkdirAll(h.cfg.MediaUploadDir, 0o755); err != nil {
			logInternal(h.logger, c, "mkdir uploads failed", err)
			return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
		}
		ext := filepath.Ext(file.Filename)
		name := uuid.NewString() + ext
		dest := filepath.Join(h.cfg.MediaUploadDir, name)
		if err := c.SaveFile(file, dest); err != nil {
			logInternal(h.logger, c, "save upload failed", err)
			return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
		}
		url = publicMediaURL(h.cfg.MediaPublicBase, c.Hostname(), name, h.cfg.IsDev())
		if title == "" {
			title = file.Filename
		}
	} else {
		assembled := mediaBody{Kind: kind, URL: url, Title: title}
		if err := reqvalid.Struct(&assembled); err != nil {
			return httperr.Write(c, fiber.StatusBadRequest, "bad_request", reqvalid.Message(err))
		}
		kind, url, title = assembled.Kind, assembled.URL, assembled.Title
	}

	order, err := h.media.NextSortOrder(c.Context(), id)
	if err != nil {
		logInternal(h.logger, c, "media sort order failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}

	m := &models.Media{
		ID:        uuid.NewString(),
		TempoID:   id,
		Kind:      kind,
		URL:       url,
		Title:     title,
		SortOrder: order,
	}
	if err := h.media.Create(c.Context(), m); err != nil {
		logInternal(h.logger, c, "create media failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	h.logOK(c, "post_media", "", "tempo_id", id, "media_id", m.ID, "kind", kind)
	return c.Status(fiber.StatusCreated).JSON(m.ToDTO())
}

// DeleteMedia — DELETE /tempos/:id/media/:mediaId
func (h *Handler) DeleteMedia(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	mediaID := c.Params("mediaId")
	h.logOp(c, "delete_media", "", "tempo_id", id, "media_id", mediaID)

	if _, err := h.tempos.FindByID(c.Context(), uid, id); err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}
	if err := h.media.Delete(c.Context(), id, mediaID); err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Media %s was not found.", mediaID))
	}
	h.logOK(c, "delete_media", "", "tempo_id", id, "media_id", mediaID)
	return c.SendStatus(fiber.StatusNoContent)
}

type reminderBody struct {
	Channels *models.ReminderChannels `json:"channels"`
	Mode     *string                  `json:"mode" validate:"omitempty,oneof=auto daily weekly"`
	Time     *string                  `json:"time" validate:"omitempty,hhmm"`
	Fun      *bool                    `json:"fun"`
}

func (b *reminderBody) Normalize() {
	if b.Mode != nil {
		m := strings.TrimSpace(*b.Mode)
		b.Mode = &m
	}
	if b.Time != nil {
		t := strings.TrimSpace(*b.Time)
		b.Time = &t
	}
}

// PatchReminder — PATCH /tempos/:id/reminder
func (h *Handler) PatchReminder(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")
	h.logOp(c, "patch_reminder", "", "tempo_id", id)

	var body reminderBody
	if err := reqvalid.Body(c, &body); err != nil {
		return err
	}

	t, err := h.tempos.FindByID(c.Context(), uid, id)
	if err != nil {
		return mapRepoErr(c, err, fmt.Sprintf("Tempo %s was not found.", id))
	}

	rem := t.Reminder
	if body.Channels != nil {
		rem.Channels = *body.Channels
	}
	if body.Mode != nil {
		rem.Mode = *body.Mode
	}
	if body.Time != nil {
		rem.Time = *body.Time
	}
	if body.Fun != nil {
		rem.Fun = *body.Fun
	}
	t.Reminder = rem

	if err := h.tempos.Save(c.Context(), t); err != nil {
		logInternal(h.logger, c, "patch reminder failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	h.logOK(c, "patch_reminder", "", "tempo_id", id, "mode", rem.Mode)
	return c.JSON(rem)
}

func (h *Handler) buildTempoDetail(c fiber.Ctx, uid string, t *models.Tempo) (*models.TempoDetail, error) {
	if err := h.tempos.LoadRelations(c.Context(), t); err != nil {
		return nil, err
	}
	return &models.TempoDetail{
		ID:           t.ID,
		Name:         t.Name,
		Content:      t.Content,
		Examples:     exampleDTOs(t.Examples),
		Tags:         tagNames(t.Tags),
		Media:        mediaDTOs(t.Media),
		Confidence:   t.Confidence,
		Due:          models.FormatDue(t.Due),
		Ease:         t.Ease,
		IntervalDays: t.IntervalDays,
		Reminder:     t.Reminder,
		Scale:        models.ClampScale(t.Scale),
	}, nil
}
