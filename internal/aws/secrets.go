package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	mdgconfig "github.com/mitdo-garden/mdg-tempo-svc/internal/config"
)

func LoadAppSecrets(ctx context.Context, cfg *mdgconfig.Config) error {
	// Skips when SecretsID is empty so environment and YAML remain sufficient.
	id := cfg.AWS.SecretsID
	if id == "" {
		return nil
	}

	opts := []func(*config.LoadOptions) error{}
	if cfg.AWS.Region != "" {
		opts = append(opts, config.WithRegion(cfg.AWS.Region))
	}
	if cfg.AWS.Endpoint != "" {
		opts = append(opts, config.WithBaseEndpoint(cfg.AWS.Endpoint))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}
	cli := secretsmanager.NewFromConfig(awsCfg)

	out, err := cli.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(id),
	})
	if err != nil {
		return fmt.Errorf("secretsmanager get %q: %w", id, err)
	}
	if out.SecretString == nil || *out.SecretString == "" {
		return fmt.Errorf("secretsmanager %q: empty secret string", id)
	}
	var data map[string]string
	if err := json.Unmarshal([]byte(*out.SecretString), &data); err != nil {
		return fmt.Errorf("secretsmanager %q: expect JSON object: %w", id, err)
	}
	cfg.ApplyAppSecrets(data)
	if cfg.MySQLDSN == "" {
		return fmt.Errorf("secretsmanager %q: secret contains no usable mysql_dsn or mysql_host/mysql_user/mysql_database fields", id)
	}
	return nil
}
