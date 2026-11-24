package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/httperr"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/models"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/reqvalid"
)

type telegramChatBody struct {
	ChatID string `json:"chatId" validate:"chatid"`
}

func (b *telegramChatBody) Normalize() {
	b.ChatID = strings.TrimSpace(b.ChatID)
}

func (h *Handler) telegramChatJSON(chatID string) fiber.Map {
	return fiber.Map{
		"chatId":      chatID,
		"botUsername": h.cfg.TelegramBotUsername,
	}
}

// GetTelegramChat — GET /settings/telegram
func (h *Handler) GetTelegramChat(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	h.logOp(c, "get_telegram_chat", "")
	chatID, err := h.telegram.ChatID(c.Context(), uid)
	if err != nil {
		logInternal(h.logger, c, "load telegram chat failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	h.logOK(c, "get_telegram_chat", "", "set", chatID != "")
	return c.JSON(h.telegramChatJSON(chatID))
}

// PatchTelegramChat — PATCH /settings/telegram
func (h *Handler) PatchTelegramChat(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	h.logOp(c, "patch_telegram_chat", "")
	var body telegramChatBody
	if err := reqvalid.Body(c, &body); err != nil {
		return err
	}
	if err := h.telegram.SaveChatID(c.Context(), uid, body.ChatID); err != nil {
		logInternal(h.logger, c, "save telegram chat failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	h.logOK(c, "patch_telegram_chat", "", "set", body.ChatID != "")
	return c.JSON(h.telegramChatJSON(body.ChatID))
}

// PostTelegramLink — POST /settings/telegram/link
// Returns a t.me link. After the user presses Start, the bot stores their chat id.
func (h *Handler) PostTelegramLink(c fiber.Ctx) error {
	uid, err := userID(c)
	if err != nil {
		return err
	}
	h.logOp(c, "link_telegram_chat", "")
	code, err := models.NewTelegramLinkCode()
	if err != nil {
		logInternal(h.logger, c, "telegram link code failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	if err := h.telegram.SaveLinkCode(c.Context(), uid, code); err != nil {
		logInternal(h.logger, c, "save telegram link failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	username := h.cfg.TelegramBotUsername
	h.logOK(c, "link_telegram_chat", "", "bot", username)
	chatID, err := h.telegram.ChatID(c.Context(), uid)
	if err != nil {
		logInternal(h.logger, c, "load telegram chat failed", err)
		return httperr.Write(c, fiber.StatusInternalServerError, "server", "Request failed.")
	}
	return c.JSON(fiber.Map{
		"chatId":      chatID,
		"botUsername": username,
		"url":         "https://t.me/" + username + "?start=" + code,
	})
}
