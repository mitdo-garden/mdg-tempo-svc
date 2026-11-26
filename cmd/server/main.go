package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/aws"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/config"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/db"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/handlers"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/pprof"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/remind"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/repository"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/server"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/telegram"
	"github.com/mitdo-garden/mdg-tempo-svc/pkg/jwks"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "err", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(cfg.MediaUploadDir, 0o755); err != nil {
		logger.Error("media upload dir failed", "err", err, "dir", cfg.MediaUploadDir)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := aws.LoadAppSecrets(ctx, cfg); err != nil {
		logger.Error("secrets manager failed", "err", err)
		os.Exit(1)
	}
	logger.Info("tempo config secrets loaded",
		"aws_secrets_id_configured", cfg.AWS.SecretsID != "",
		"mysql_configured", cfg.MySQLDSN != "",
	)

	jwksClient, err := jwks.NewClient(cfg.JWKSURL)
	if err != nil {
		logger.Error("jwks client failed", "err", err)
		os.Exit(1)
	}
	verifier := jwks.NewVerifier(jwksClient, cfg.JWTIssuer)

	if err := cfg.ValidateMySQL(); err != nil {
		logger.Error("mysql config invalid", "err", err)
		os.Exit(1)
	}

	gdb, err := db.Connect(cfg.MySQLDSN)
	if err != nil {
		logger.Error("database connect failed", "err", err)
		os.Exit(1)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		logger.Error("database pool failed", "err", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	tempos := repository.NewTempoRepository(gdb)
	examples := repository.NewExampleRepository(gdb)
	media := repository.NewMediaRepository(gdb)
	tags := repository.NewTagRepository(gdb)
	history := repository.NewHistoryRepository(gdb)
	telegramChats := repository.NewTelegramRepository(gdb)

	h := handlers.New(cfg, tempos, examples, media, tags, history, telegramChats, logger)
	app := server.New(cfg, h, verifier, logger)

	pprof.Listen(ctx, cfg.PprofAddr, logger)
	if cfg.TelegramBotToken != "" {
		tg := telegram.New(cfg.TelegramBotToken)
		tg.OnClaim(func(ctx context.Context, chatID, code string) (bool, error) {
			ok, err := telegramChats.ClaimLink(ctx, code, chatID)
			if err != nil {
				logger.Error("telegram link claim failed", "err", err)
				return false, err
			}
			if ok {
				logger.Info("telegram chat linked", "chat_id", chatID)
			}
			return ok, nil
		})
		job := remind.New(tempos, tags, history, tg, cfg.RemindEvery, logger)
		go job.Run(ctx)
		go tg.Listen(ctx, logger)
		logger.Info("telegram reminder started",
			"bot", cfg.TelegramBotUsername,
			"name", cfg.TelegramBotName,
			"interval", cfg.RemindEvery.String(),
		)
	} else {
		logger.Info("telegram reminder disabled", "reason", "missing bot token")
	}

	go func() {
		addr := ":" + cfg.Port
		logger.Info("mdg-tempo-svc listening",
			"addr", addr,
			"env", cfg.AppEnv,
			"app", cfg.AppName,
			"jwks_url", cfg.JWKSURL,
			"jwt_issuer", cfg.JWTIssuer,
			"framework", "fiber",
		)
		if err := app.Listen(addr); err != nil {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancel()

	if err := app.Shutdown(); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("mdg-tempo-svc stopped")
}
