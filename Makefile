.PHONY: up down logs test test-unit test-integration test-e2e test-frontend test-site check-deploy db-test-up db-test-down

TEST_DATABASE_URL ?= postgres://test:test@localhost:55432/smashhub_test?sslmode=disable

INSTANCE1_URL ?= http://localhost:8080
INSTANCE2_URL ?= http://localhost:8090

## ローカルで2つのインスタンスを起動する（Instance1: http://localhost:8080 / Instance2: http://localhost:8090）
up:
	docker compose up --build -d --wait
	@echo "Instance1: $(INSTANCE1_URL) / Instance2: $(INSTANCE2_URL) で起動しました"

down:
	docker compose down

logs:
	docker compose logs -f backend1 backend2 web1 web2

## すべてのテスト
test: test-unit test-integration test-frontend test-site test-e2e

## バックエンドの単体テスト（DB 不要）
test-unit:
	cd backend && go test ./...

## バックエンドの結合テスト（PostgreSQL を Docker で起動して実行）
test-integration: db-test-up
	cd backend && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -tags=integration -count=1 ./internal/integrationtest/...

## 2つのインスタンスを Docker で起動し、SmashHubプロトコルでの対戦の流れを検証する（E2E テスト）
test-e2e: up
	cd backend && INSTANCE1_URL="$(INSTANCE1_URL)" INSTANCE2_URL="$(INSTANCE2_URL)" go test -tags=e2e -count=1 ./e2e/...

db-test-up:
	docker compose --profile test up -d --wait db-test

db-test-down:
	docker compose --profile test rm -sf db-test

## 紹介サイト（site/、GitHub Pages）の型チェックとビルド
test-site:
	cd site && npm ci && npm run build

## フロントエンドの型チェックとビルド
test-frontend:
	cd frontend && npm ci && npm run build

## 本番用の構成（deploy/）の検証。compose はダミーの値で設定を検証し、スクリプトは構文と shellcheck（入っていれば）を確認する
DEPLOY_DUMMY_ENV = DOMAIN=example.com DB_PASSWORD=dummy INSTANCE_PRIVATE_KEY=dummy IDENTIFIER_PEPPER=dummy ACCOUNT_KEY_SECRET=dummy

check-deploy:
	$(DEPLOY_DUMMY_ENV) docker compose -f deploy/compose.yaml --env-file /dev/null config -q
	bash -n deploy/setup.sh
	bash -n deploy/smash-hub.sh
	@if command -v shellcheck >/dev/null 2>&1; then shellcheck deploy/setup.sh deploy/smash-hub.sh; else echo "shellcheck がないため省略しました"; fi
	docker run --rm -e DOMAIN=example.com -v "$(CURDIR)/deploy/Caddyfile:/etc/caddy/Caddyfile:ro" caddy:2-alpine caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
