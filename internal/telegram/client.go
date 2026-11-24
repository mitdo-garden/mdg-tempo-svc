package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client sends plain-text messages with a bot token.
type Client struct {
	token string
	base  string
	http  *http.Client

	mu     sync.Mutex
	offset int64
	claim  func(ctx context.Context, chatID, code string) (bool, error)
}

func New(token string) *Client {
	return &Client{
		token: strings.TrimSpace(token),
		base:  "https://api.telegram.org",
		http:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Send posts text to one chat.
func (c *Client) Send(ctx context.Context, chatID, text string) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return fmt.Errorf("telegram: chat id is required")
	}
	payload, err := json.Marshal(map[string]string{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	})
	if err != nil {
		return fmt.Errorf("telegram: encode message: %w", err)
	}
	var out apiResponse
	if err := c.post(ctx, "sendMessage", payload, &out); err != nil {
		return err
	}
	if !out.OK {
		return fmt.Errorf("telegram: %s", blank(out.Description, "send failed"))
	}
	return nil
}

// OnClaim connects a /start <code> payload to the user who issued that code.
func (c *Client) OnClaim(fn func(ctx context.Context, chatID, code string) (bool, error)) {
	c.claim = fn
}

// Listen replies to /start and /chatid with that chat's numeric id.
func (c *Client) Listen(ctx context.Context, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	c.reply(ctx, logger)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.reply(ctx, logger)
		}
	}
}

func (c *Client) reply(ctx context.Context, logger *slog.Logger) {
	if err := c.ReplyChatIDs(ctx); err != nil && ctx.Err() == nil {
		logger.Error("telegram chat id reply failed", "err", err)
	}
}

// ReplyChatIDs reads pending updates and answers chat-id commands.
func (c *Client) ReplyChatIDs(ctx context.Context) error {
	c.mu.Lock()
	offset := c.offset
	c.mu.Unlock()

	payload := map[string]any{"limit": 50, "timeout": 0}
	if offset > 0 {
		payload["offset"] = offset
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var out updatesResponse
	if err := c.post(ctx, "getUpdates", body, &out); err != nil {
		return err
	}
	if !out.OK {
		return fmt.Errorf("telegram: %s", blank(out.Description, "getUpdates failed"))
	}
	var next int64
	for _, u := range out.Result {
		if u.UpdateID+1 > next {
			next = u.UpdateID + 1
		}
		id, _, ok := u.chat()
		if !ok || u.Message == nil {
			continue
		}
		cmd, code, isCmd := splitCommand(u.Message.Text)
		if !isCmd {
			continue
		}
		text := "Your chat id is " + id
		if cmd == "/start" && code != "" && c.claim != nil {
			claimed, err := c.claim(ctx, id, code)
			if err != nil {
				return err
			}
			if claimed {
				text = "Connected. Your chat id is " + id
			}
		}
		if err := c.Send(ctx, id, text); err != nil {
			return err
		}
	}
	if next > offset {
		c.mu.Lock()
		if next > c.offset {
			c.offset = next
		}
		c.mu.Unlock()
	}
	return nil
}

func splitCommand(text string) (cmd, payload string, ok bool) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return "", "", false
	}
	cmd = strings.ToLower(fields[0])
	if i := strings.IndexByte(cmd, '@'); i >= 0 {
		cmd = cmd[:i]
	}
	if len(fields) > 1 {
		payload = fields[1]
	}
	switch cmd {
	case "/start", "/chatid", "/id":
		return cmd, payload, true
	default:
		return "", "", false
	}
}

func (c *Client) post(ctx context.Context, method string, body []byte, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/bot"+c.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return c.scrub(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return c.scrub(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return c.scrub(err)
	}
	if err := decode(raw, dest); err != nil {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("telegram: %s", resp.Status)
		}
		return fmt.Errorf("telegram: decode response: %w", err)
	}
	return nil
}

func (c *Client) scrub(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if c.token != "" {
		msg = strings.ReplaceAll(msg, c.token, "redacted")
	}
	return fmt.Errorf("telegram: %s", msg)
}

func decode(raw []byte, dest any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(dest)
}

func blank(s, fallback string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	return s
}

type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

type updatesResponse struct {
	OK          bool     `json:"ok"`
	Description string   `json:"description"`
	Result      []update `json:"result"`
}

type update struct {
	UpdateID      int64             `json:"update_id"`
	Message       *tgMessage        `json:"message"`
	EditedMessage *tgMessage        `json:"edited_message"`
	ChannelPost   *tgMessage        `json:"channel_post"`
	MyChatMember  *chatMemberUpdate `json:"my_chat_member"`
	CallbackQuery *callbackQuery    `json:"callback_query"`
}

type tgMessage struct {
	Text string `json:"text"`
	Chat tgChat `json:"chat"`
}

type chatMemberUpdate struct {
	Chat tgChat `json:"chat"`
}

type callbackQuery struct {
	Message *tgMessage `json:"message"`
}

type tgChat struct {
	ID   json.Number `json:"id"`
	Type string      `json:"type"`
}

func (u update) chat() (id, kind string, ok bool) {
	switch {
	case u.Message != nil && u.Message.Chat.ID != "":
		return u.Message.Chat.ID.String(), u.Message.Chat.Type, true
	case u.EditedMessage != nil && u.EditedMessage.Chat.ID != "":
		return u.EditedMessage.Chat.ID.String(), u.EditedMessage.Chat.Type, true
	case u.ChannelPost != nil && u.ChannelPost.Chat.ID != "":
		return u.ChannelPost.Chat.ID.String(), u.ChannelPost.Chat.Type, true
	case u.MyChatMember != nil && u.MyChatMember.Chat.ID != "":
		return u.MyChatMember.Chat.ID.String(), u.MyChatMember.Chat.Type, true
	case u.CallbackQuery != nil && u.CallbackQuery.Message != nil && u.CallbackQuery.Message.Chat.ID != "":
		return u.CallbackQuery.Message.Chat.ID.String(), u.CallbackQuery.Message.Chat.Type, true
	default:
		return "", "", false
	}
}
