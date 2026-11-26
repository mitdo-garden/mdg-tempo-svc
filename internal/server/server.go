package server

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/config"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/handlers"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/middleware"
	"github.com/mitdo-garden/mdg-tempo-svc/pkg/jwks"
)

// New builds the Fiber app with Review / Tempo / Map routes.
func New(cfg *config.Config, h *handlers.Handler, verifier *jwks.Verifier, log *slog.Logger) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "mdg-tempo-svc",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			middleware.Logger(c, log).Error("request error", "err", err, "path", c.Path(), "status", code)
			return c.Status(code).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "server",
					"message": "Request failed.",
				},
			})
		},
	})

	app.Use(requestid.New())
	app.Use(recover.New())
	// Structured access log (request_id, user_id, route, params, latency). Replaces Fiber's text logger.
	app.Use(middleware.RequestLog(log))
	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			return originAllowed(origin, cfg.FrontendURL, cfg.IsDev())
		},
		AllowMethods: []string{
			fiber.MethodGet,
			fiber.MethodPost,
			fiber.MethodPut,
			fiber.MethodPatch,
			fiber.MethodDelete,
			fiber.MethodOptions,
		},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	app.Use(func(c fiber.Ctx) error {
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	})

	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "mdg-tempo-svc"})
	})

	// Uploaded media files (multipart POST /tempos/:id/media).
	app.Use("/media", static.New(cfg.MediaUploadDir))

	requireAuth := middleware.Authenticate(verifier, cfg.CookieName)

	register := func(r fiber.Router) {
		r.Use(requireAuth)

		// Map
		r.Get("/map", h.GetMap)

		// Review
		r.Get("/review", h.GetReview)
		r.Get("/review/:id", h.GetReviewTempo)
		r.Post("/:id/reviews", h.PostReview)
		r.Post("/:id/examples/generate", h.GenerateExample)
		r.Post("/:id/examples", h.PostExample)
		r.Patch("/:id/examples/:exampleId", h.PatchExample)
		r.Delete("/:id/examples/:exampleId", h.DeleteExample)

		// Tempo detail
		r.Post("", h.PostTempo)
		r.Get("/tags", h.GetTags)
		r.Get("/settings/telegram", h.GetTelegramChat)
		r.Patch("/settings/telegram", h.PatchTelegramChat)
		r.Post("/settings/telegram/link", h.PostTelegramLink)
		r.Get("/:id", h.GetTempo)
		r.Patch("/:id", h.PatchTempo)
		r.Delete("/:id", h.DeleteTempo)
		r.Put("/:id/tags", h.PutTags)
		r.Post("/:id/media", h.PostMedia)
		r.Delete("/:id/media/:mediaId", h.DeleteMedia)
		r.Patch("/:id/reminder", h.PatchReminder)
	}

	register(app.Group("/tempos"))

	return app
}
