package remind

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/repository"
)

type picker interface {
	NextToRemind(ctx context.Context) (*repository.ReminderTarget, error)
}

type tagger interface {
	TagsByTempoIDs(ctx context.Context, tempoIDs []string) (map[string][]string, error)
}

type reminderLog interface {
	AppendReminder(ctx context.Context, userID string, tempoID string, at time.Time) error
}

// Sender delivers one plain-text Telegram message to a chat id.
type Sender interface {
	Send(ctx context.Context, chatID, text string) error
}

// Job picks one tempo per tick and pushes its name and tags to Telegram.
type Job struct {
	tempos  picker
	tags    tagger
	history reminderLog
	sender  Sender
	every   time.Duration
	logger  *slog.Logger
	now     func() time.Time
}

func New(tempos picker, tags tagger, history reminderLog, sender Sender, every time.Duration, logger *slog.Logger) *Job {
	if every <= 0 {
		every = time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Job{
		tempos:  tempos,
		tags:    tags,
		history: history,
		sender:  sender,
		every:   every,
		logger:  logger,
		now:     time.Now,
	}
}

// Run sends once immediately, then every interval, until ctx is cancelled.
func (j *Job) Run(ctx context.Context) {
	j.tick(ctx)
	ticker := time.NewTicker(j.every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.tick(ctx)
		}
	}
}

func (j *Job) tick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	tempo, err := j.tempos.NextToRemind(ctx)
	if errors.Is(err, repository.ErrNotFound) {
		j.logger.Info("reminder skipped", "reason", "no tempos")
		return
	}
	if err != nil {
		j.logger.Error("reminder pick failed", "err", err)
		return
	}

	byID, err := j.tags.TagsByTempoIDs(ctx, []string{tempo.ID})
	if err != nil {
		j.logger.Error("reminder tags failed", "err", err, "tempo_id", tempo.ID)
		return
	}
	chatID := strings.TrimSpace(tempo.ChatID)
	if chatID == "" {
		j.logger.Info("reminder skipped", "reason", "no telegram chat id", "tempo_id", tempo.ID)
		return
	}
	text := Message(tempo.Name, byID[tempo.ID])
	if text == "" {
		j.logger.Info("reminder skipped", "reason", "empty name and tags", "tempo_id", tempo.ID)
		return
	}
	if err := j.sender.Send(ctx, chatID, text); err != nil {
		j.logger.Error("telegram send failed", "err", err, "tempo_id", tempo.ID)
		return
	}
	if err := j.history.AppendReminder(ctx, tempo.UserID, tempo.ID, j.now().UTC()); err != nil {
		j.logger.Error("reminder log failed", "err", err, "tempo_id", tempo.ID)
		return
	}
	j.logger.Info("reminder sent", "tempo_id", tempo.ID, "name", tempo.Name)
}
