# mdg-tempo-svc

Garden data API for **Mitdo** — tempos and reviews. Same stack as `mdg-auth-svc`: **Go (Fiber) + GORM/MySQL + Viper YAML**. Access JWTs from auth-svc are verified via **JWKS** (no private keys here).

Tracking and settings stay client-side for now. This service implements **Review**, **Tempo detail**, **Map**, and tempo **create / update / delete**. Organize tempos with **tags** — there is no collection or link concept.

## Folder structure

```
mdg-tempo-svc/
├── configs/
│   ├── config.yaml
│   ├── config.dev.yaml
│   ├── config.test.yaml
│   └── config.prd.yaml
├── cmd/server/main.go
├── migrations/001_init.sql # CREATE TABLE only
├── migrations/002_drop_position.sql # drop pos_x/pos_y; keeps existing rows
├── pkg/jwks/              # consumer JWKS client (copied from auth-svc)
└── internal/
    ├── auth/              # JWT claims shape
    ├── config/
    ├── db/
    ├── handlers/
    ├── middleware/
    ├── models/
    ├── repository/
    ├── schedule/          # SM-2-lite (matches UI)
    └── server/
```

## Run

Requires MySQL and a running `mdg-auth-svc` (for JWKS + session cookie).

```bash
make migrate && make run-dev   # :8081
```

Profiles merge like Spring: `config.yaml` → `config.<APP_ENV>.yaml` → extras from `app.profiles` / `APP_PROFILES`. Overlays are deltas only. Values use `${NAME:default}` placeholders.

| File | When |
|------|------|
| `config.yaml` | Shared defaults |
| `config.dev.yaml` | `APP_ENV=dev` (default) — port **8081**, local MySQL/JWKS |
| `config.test.yaml` | `APP_ENV=test` — port **5003**, DB `mdg_tempo_test` |
| `config.prd.yaml` | `APP_ENV=prd` — port only; inject the rest via env |

Auth JWKS URL (dev): `http://localhost:8080/.well-known/jwks.json`. Cookie name: `mitdo_session` (same as auth-svc). CORS allows `frontend.url` with credentials.

## Endpoints

All routes (except `/healthz` and `/media/*`) require a valid access JWT via `Authorization: Bearer` or the `mitdo_session` cookie. Dual mount: `/…` and `/v1/…`.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Liveness |
| `GET` | `/review` | Lightweight queue (`items`: id, name, tags, confidence, due) |
| `GET` | `/review/tempos/:id` | Review card payload (content, examples, media, tags) |
| `POST` | `/tempos/:id/reviews` | Grade 1–5; updates schedule + history |
| `POST` | `/tempos/:id/examples` | Add example `{ text }` |
| `POST` | `/tempos/:id/examples/generate` | Stub prompt from name/content |
| `DELETE` | `/tempos/:id/examples/:exampleId` | Remove example |
| `POST` | `/tempos` | Create tempo (`name`, optional `id`) |
| `GET` | `/tempos/:id` | Full tempo |
| `PATCH` | `/tempos/:id` | Update `name`, `content`, `scale` |
| `DELETE` | `/tempos/:id` | Delete tempo |
| `PUT` | `/tempos/:id/tags` | Replace tags (empty stays empty) |
| `GET` | `/tags?q=` | Tag search |
| `POST` | `/tempos/:id/media` | JSON `{ kind, url, title }` or multipart `file` |
| `DELETE` | `/tempos/:id/media/:mediaId` | Remove media |
| `PATCH` | `/tempos/:id/reminder` | Reminder channels / mode / time / fun |
| `GET` | `/map` | List stubs (`q`, `due`, `tag`, `limit` 1–100 default 40, `offset` ≥ 0); due + low confidence first; `total`/`hasMore`/`dueCount`. Detail via `GET /tempos/:id` |

**Not implemented yet:** `GET /summary`, `GET /tracking`, `GET|PATCH /settings`.

Errors use `{ "error": { "code", "message" } }` — `400` / `401` / `404` / `409` / `422`.

Request query/body params use structs with `validate` tags (`internal/reqvalid`, go-playground/validator). Invalid input → `400` (not silent clamping).

## Auth

```go
cl, _ := jwks.NewClient(cfg.JWKSURL)
v := jwks.NewVerifier(cl, "mdg-auth-svc")
```

Issuer must match auth-svc (`jwt.issuer`). Claims include `uid` (garden owner).
