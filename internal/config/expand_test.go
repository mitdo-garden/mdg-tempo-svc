package config

import "testing"

func TestExpandStringDefault(t *testing.T) {
	got := ExpandString("${DOES_NOT_EXIST_XYZ:mdg_tempo}")
	if got != "mdg_tempo" {
		t.Fatalf("got %q", got)
	}
}

func TestExpandStringEnvWins(t *testing.T) {
	t.Setenv("DATABASE_NAME", "from_env")
	got := ExpandString("${DATABASE_NAME:mdg_tempo}")
	if got != "from_env" {
		t.Fatalf("got %q", got)
	}
}

func TestExpandStringURLDefault(t *testing.T) {
	got := ExpandString("${JWKS_URL_UNSET:http://localhost:8080/.well-known/jwks.json}")
	if got != "http://localhost:8080/.well-known/jwks.json" {
		t.Fatalf("got %q", got)
	}
}

func TestExpandStringNested(t *testing.T) {
	t.Setenv("AWS_REGION", "ap-southeast-1")
	got := ExpandString("${MEDIA_REGION_UNSET:${AWS_REGION:us-east-1}}")
	if got != "ap-southeast-1" {
		t.Fatalf("got %q", got)
	}
}

func TestExpandStringDefaultMayContainColon(t *testing.T) {
	got := ExpandString("${BOT_TOKEN_UNSET:123:abc}")
	if got != "123:abc" {
		t.Fatalf("got %q", got)
	}
}

func TestExpandStringEmptyDefault(t *testing.T) {
	got := ExpandString("${UNSET_TOKEN:}")
	if got != "" {
		t.Fatalf("got %q", got)
	}
}
