package httperr

import "github.com/gofiber/fiber/v3"

// Write sends the Mitdo error envelope: { "error": { "code", "message" } }.
func Write(c fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    code,
			"message": message,
		},
	})
}
