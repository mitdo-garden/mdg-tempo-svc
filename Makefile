.PHONY: run run-dev run-test run-prd build tidy migrate migrate-test test

APP_ENV ?= dev
# Go runtime caps for the server process. Override: make run GOMAXPROCS=2 GOMEMLIMIT=1GiB
GOMAXPROCS ?= 4
GOMEMLIMIT ?= 4GiB

run: run-dev

run-dev: build
	@printf 'mdg-tempo-svc limits: %s CPU, %s memory\n' "$(GOMAXPROCS)" "$(GOMEMLIMIT)"
	APP_ENV=dev GOMAXPROCS=$(GOMAXPROCS) GOMEMLIMIT=$(GOMEMLIMIT) ./bin/mdg-tempo-svc

run-test: build
	@printf 'mdg-tempo-svc limits: %s CPU, %s memory\n' "$(GOMAXPROCS)" "$(GOMEMLIMIT)"
	APP_ENV=test GOMAXPROCS=$(GOMAXPROCS) GOMEMLIMIT=$(GOMEMLIMIT) ./bin/mdg-tempo-svc

run-prd: build
	@printf 'mdg-tempo-svc limits: %s CPU, %s memory\n' "$(GOMAXPROCS)" "$(GOMEMLIMIT)"
	APP_ENV=prd GOMAXPROCS=$(GOMAXPROCS) GOMEMLIMIT=$(GOMEMLIMIT) ./bin/mdg-tempo-svc

build:
	go build -o bin/mdg-tempo-svc ./cmd/server

tidy:
	go mod tidy

test:
	go test ./...

migrate:
	mysql -h 127.0.0.1 -P 3306 -uroot -proot -e "CREATE DATABASE IF NOT EXISTS mdg_tempo CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
	@n=$$(mysql -h 127.0.0.1 -P 3306 -uroot -proot -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mdg_tempo' AND table_name='tempos'"); \
	if [ "$$n" = "0" ]; then mysql -h 127.0.0.1 -P 3306 -uroot -proot mdg_tempo < migrations/001_init.sql; fi
	mysql -h 127.0.0.1 -P 3306 -uroot -proot mdg_tempo < migrations/002_drop_position.sql
	mysql -h 127.0.0.1 -P 3306 -uroot -proot mdg_tempo < migrations/003_user_telegram.sql
	mysql -h 127.0.0.1 -P 3306 -uroot -proot mdg_tempo < migrations/004_telegram_link_code.sql

migrate-test:
	mysql -h 127.0.0.1 -P 3306 -uroot -proot -e "CREATE DATABASE IF NOT EXISTS mdg_tempo_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
	@n=$$(mysql -h 127.0.0.1 -P 3306 -uroot -proot -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mdg_tempo_test' AND table_name='tempos'"); \
	if [ "$$n" = "0" ]; then mysql -h 127.0.0.1 -P 3306 -uroot -proot mdg_tempo_test < migrations/001_init.sql; fi
	mysql -h 127.0.0.1 -P 3306 -uroot -proot mdg_tempo_test < migrations/002_drop_position.sql
	mysql -h 127.0.0.1 -P 3306 -uroot -proot mdg_tempo_test < migrations/003_user_telegram.sql
	mysql -h 127.0.0.1 -P 3306 -uroot -proot mdg_tempo_test < migrations/004_telegram_link_code.sql
