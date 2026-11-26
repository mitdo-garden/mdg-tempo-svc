package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	EnvDev  = "dev"
	EnvTest = "test"
	EnvPrd  = "prd"
)

// AWSConfig holds the configuration for AWS services.
type AWSConfig struct {
	Region    string
	SecretsID string
	Endpoint  string
}

// Config is loaded from configs/*.yaml with ${ENV:default} placeholders
// (APP_ENV selects the overlay).
type Config struct {
	AppName         string
	AppEnv          string
	Port            string
	MySQLDSN        string
	JWTIssuer       string
	JWKSURL         string
	FrontendURL     string
	CookieName      string
	MediaUploadDir  string
	MediaPublicBase string
	ProjectRoot     string

	TelegramBotToken    string
	TelegramBotUsername string
	TelegramBotName     string
	TelegramChatID      string
	RemindEvery         time.Duration
	PprofAddr           string
	AWS                 AWSConfig
}

func Load() (*Config, error) {
	env := NormalizeEnv(os.Getenv("APP_ENV"))
	root := findProjectRoot()

	v := viper.New()
	v.AddConfigPath(filepath.Join(root, "configs"))
	v.SetConfigType("yaml")

	v.SetConfigName("config")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read configs/config.yaml: %w", err)
	}

	if err := mergeEnvAndProfiles(v, env); err != nil {
		return nil, err
	}

	// Spring-style ${NAME:default} from process env into all string leaves.
	expanded := expandSettings(v.AllSettings())
	mergeExpanded(v.Set, "", expanded)

	uploadDir := firstNonEmpty(v.GetString("media.upload_dir"), "uploads")
	if !filepath.IsAbs(uploadDir) {
		uploadDir = filepath.Join(root, uploadDir)
	}

	every, err := remindEvery(v.GetString("telegram.interval"))
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		AppName:             firstNonEmpty(v.GetString("app.name"), "mdg-tempo-svc"),
		AppEnv:              env,
		Port:                firstNonEmpty(v.GetString("server.port"), "5002"),
		MySQLDSN:            mysqlDSN(v),
		JWTIssuer:           firstNonEmpty(v.GetString("jwt.issuer"), "mdg-auth-svc"),
		JWKSURL:             strings.TrimSpace(v.GetString("jwt.jwks_url")),
		FrontendURL:         strings.TrimRight(v.GetString("frontend.url"), "/"),
		CookieName:          firstNonEmpty(v.GetString("cookie.name"), "mitdo_session"),
		MediaUploadDir:      uploadDir,
		MediaPublicBase:     strings.TrimRight(firstNonEmpty(v.GetString("media.public_base_url"), "http://localhost:8081/media"), "/"),
		ProjectRoot:         root,
		TelegramBotToken:    strings.TrimSpace(v.GetString("telegram.bot_token")),
		TelegramBotUsername: strings.TrimPrefix(strings.TrimSpace(firstNonEmpty(v.GetString("telegram.bot_username"), "mitdobot")), "@"),
		TelegramBotName:     firstNonEmpty(v.GetString("telegram.bot_name"), "Mitdo"),
		TelegramChatID:      strings.TrimSpace(v.GetString("telegram.chat_id")),
		RemindEvery:         every,
		PprofAddr:           strings.TrimSpace(v.GetString("pprof.addr")),
		AWS: AWSConfig{
			Region:    strings.TrimSpace(v.GetString("kms.region")),
			SecretsID: strings.TrimSpace(v.GetString("kms.secrets_id")),
			Endpoint:  strings.TrimSpace(v.GetString("kms.endpoint")),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config (%s): %w", env, err)
	}
	return cfg, nil
}

func mysqlDSN(v *viper.Viper) string {
	if dsn := strings.TrimSpace(v.GetString("mysql.dsn")); dsn != "" {
		return dsn
	}
	host := v.GetString("mysql.host")
	port := firstNonEmpty(v.GetString("mysql.port"), "3306")
	user := v.GetString("mysql.user")
	pass := v.GetString("mysql.password")
	dbName := v.GetString("mysql.database")
	params := firstNonEmpty(v.GetString("mysql.params"), "parseTime=true&loc=UTC")
	if host == "" || user == "" || dbName == "" {
		return ""
	}
	return fmt.Sprintf("%s@tcp(%s:%s)/%s?%s",
		url.UserPassword(user, pass).String(), host, port, dbName, params)
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.FrontendURL) == "" {
		return fmt.Errorf("frontend.url is required in YAML")
	}
	if strings.TrimSpace(c.JWKSURL) == "" {
		return fmt.Errorf("jwt.jwks_url is required in YAML")
	}
	if strings.TrimSpace(c.JWTIssuer) == "" {
		return fmt.Errorf("jwt.issuer is required in YAML")
	}
	return nil
}

// ValidateMySQL checks MySQL configuration after environment and secrets have been applied.
func (c *Config) ValidateMySQL() error {
	if strings.TrimSpace(c.MySQLDSN) == "" {
		return fmt.Errorf("mysql configuration is required (MYSQL_DSN or MYSQL_* env or secrets backend)")
	}
	return nil
}

func (c *Config) IsDev() bool  { return c.AppEnv == EnvDev }
func (c *Config) IsTest() bool { return c.AppEnv == EnvTest }
func (c *Config) IsPrd() bool  { return c.AppEnv == EnvPrd }

func NormalizeEnv(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "dev", "development", "local":
		return EnvDev
	case "test", "testing", "ci":
		return EnvTest
	case "prd", "prod", "production":
		return EnvPrd
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func remindEvery(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Minute, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("telegram.interval: %w", err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("telegram.interval must be positive")
	}
	return d, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func findProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "configs", "config.yaml")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}

// ApplyAppSecrets fills Tempo fields from an AWS Secrets Manager JSON object.
// Existing environment/YAML values win.
func (c *Config) ApplyAppSecrets(data map[string]string) {
	fill := func(dst *string, key string) {
		if strings.TrimSpace(*dst) != "" {
			return
		}
		if value := strings.TrimSpace(data[key]); value != "" {
			*dst = value
		}
	}

	fill(&c.FrontendURL, "frontend_url")
	fill(&c.JWKSURL, "jwks_url")
	fill(&c.JWTIssuer, "jwt_issuer")
	fill(&c.MediaPublicBase, "media_public_base_url")
	fill(&c.TelegramBotToken, "telegram_bot_token")
	fill(&c.TelegramBotUsername, "telegram_bot_username")
	fill(&c.TelegramBotName, "telegram_bot_name")
	fill(&c.TelegramChatID, "telegram_chat_id")

	if strings.TrimSpace(c.MySQLDSN) == "" {
		if dsn := strings.TrimSpace(data["mysql_dsn"]); dsn != "" {
			c.MySQLDSN = dsn
			return
		}
		host := strings.TrimSpace(data["mysql_host"])
		port := firstNonEmpty(strings.TrimSpace(data["mysql_port"]), "3306")
		user := strings.TrimSpace(data["mysql_user"])
		password := strings.TrimSpace(data["mysql_password"])
		database := strings.TrimSpace(data["mysql_database"])
		params := firstNonEmpty(strings.TrimSpace(data["mysql_params"]), "parseTime=true&loc=UTC")
		if host != "" && user != "" && database != "" {
			c.MySQLDSN = fmt.Sprintf("%s@tcp(%s:%s)/%s?%s",
				url.UserPassword(user, password).String(), host, port, database, params)
		}
	}
}
