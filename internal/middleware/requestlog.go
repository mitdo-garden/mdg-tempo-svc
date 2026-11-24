package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

const LoggerKey = "logger"

// RequestLog attaches a request-scoped slog.Logger and emits one access line per API call.
// Place after requestid.New() so request_id is set. Auth middleware may run before or after;
// user_id is included when claims are present by the time the handler returns.
//
// Top-level fields stay stable for filtering; route params / query / auth go under contextData.
func RequestLog(base *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		rid := requestid.FromContext(c)
		log := base.With("request_id", rid)
		c.Locals(LoggerKey, log)

		start := time.Now()
		err := c.Next()

		status := c.StatusCode()
		attrs := []any{
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"latency_ms", time.Since(start).Milliseconds(),
		}
		ctx := make([]any, 0, 8)
		if route := c.Route(); route != nil && route.Path != "" {
			attrs = append(attrs, "route", route.Path)
			for _, name := range route.Params {
				if v := c.Params(name); v != "" {
					ctx = append(ctx, "param_"+name, v)
				}
			}
		}
		if uid, ok := UserID(c); ok {
			attrs = append(attrs, "user_id", uid)
		}
		if src, ok := c.Locals("authSource").(string); ok && src != "" {
			ctx = append(ctx, "auth", src)
		}
		for k, v := range c.Queries() {
			if v != "" {
				ctx = append(ctx, "q_"+k, v)
			}
		}
		if len(ctx) > 0 {
			attrs = append(attrs, slog.Group("contextData", ctx...))
		}

		msg := "api"
		switch {
		case status >= 500:
			log.Error(msg, attrs...)
		case status >= 400:
			log.Warn(msg, attrs...)
		default:
			log.Info(msg, attrs...)
		}
		return err
	}
}

// Logger returns the request-scoped logger, or base if unset.
func Logger(c fiber.Ctx, base *slog.Logger) *slog.Logger {
	if l, ok := c.Locals(LoggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return base
}
