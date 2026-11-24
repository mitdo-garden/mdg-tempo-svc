package remind

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/repository"
)

type fakePick struct {
	t   *repository.ReminderTarget
	err error
}

func (f fakePick) NextToRemind(context.Context) (*repository.ReminderTarget, error) {
	return f.t, f.err
}

type fakeTags struct {
	m   map[string][]string
	err error
}

func (f fakeTags) TagsByTempoIDs(context.Context, []string) (map[string][]string, error) {
	return f.m, f.err
}

type fakeLog struct {
	userID  string
	tempoID string
	err     error
	calls   int
}

func (f *fakeLog) AppendReminder(_ context.Context, userID string, tempoID string, _ time.Time) error {
	f.calls++
	f.userID = userID
	f.tempoID = tempoID
	return f.err
}

type fakeSend struct {
	texts   []string
	chatIDs []string
	err     error
}

func (f *fakeSend) Send(_ context.Context, chatID, text string) error {
	if f.err != nil {
		return f.err
	}
	f.chatIDs = append(f.chatIDs, chatID)
	f.texts = append(f.texts, text)
	return nil
}

func testJob(p fakePick, tags fakeTags, log *fakeLog, send *fakeSend) *Job {
	return New(p, tags, log, send, time.Minute, slog.New(slog.DiscardHandler))
}

func TestTickSendsNameAndTag(t *testing.T) {
	send := &fakeSend{}
	log := &fakeLog{}
	j := testJob(
		fakePick{t: &repository.ReminderTarget{ID: "t1", UserID: "u1", Name: "Phở", ChatID: "555"}},
		fakeTags{m: map[string][]string{"t1": {"food"}}},
		log,
		send,
	)
	j.tick(context.Background())
	if len(send.texts) != 1 || send.texts[0] != "<blockquote>🌿  <b>Phở</b>\n<i>food</i></blockquote>" || send.chatIDs[0] != "555" {
		t.Fatalf("sent %#v chats %#v", send.texts, send.chatIDs)
	}
	if log.calls != 1 || log.tempoID != "t1" || log.userID != "u1" {
		t.Fatalf("log %+v", log)
	}
}

func TestTickSkipsWhenEmpty(t *testing.T) {
	send := &fakeSend{}
	log := &fakeLog{}
	j := testJob(fakePick{err: repository.ErrNotFound}, fakeTags{}, log, send)
	j.tick(context.Background())
	if len(send.texts) != 0 || log.calls != 0 {
		t.Fatalf("sent %#v log %d", send.texts, log.calls)
	}
}

func TestTickDoesNotLogWhenSendFails(t *testing.T) {
	send := &fakeSend{err: errors.New("down")}
	log := &fakeLog{}
	j := testJob(
		fakePick{t: &repository.ReminderTarget{ID: "t1", UserID: "u1", Name: "Walk", ChatID: "555"}},
		fakeTags{m: map[string][]string{"t1": {"health"}}},
		log,
		send,
	)
	j.tick(context.Background())
	if log.calls != 0 {
		t.Fatal("logged a failed send")
	}
}
