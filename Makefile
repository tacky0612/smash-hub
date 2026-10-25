.PHONY: up down logs test test-unit test-integration test-frontend db-test-up db-test-down

TEST_DATABASE_URL ?= postgres://test:test@localhost:55432/smashhub_test?sslmode=disable

## ローカルで全体を起動する（http://localhost:8080）
up:
	docker compose up --build -d
	@echo "http://localhost:8080 で起動しました"

down:
	docker compose down

logs:
	docker compose logs -f backend web

## すべてのテスト
test: test-unit test-integration test-frontend

## バックエンドの単体テスト（DB 不要）
test-unit:
	cd backend && go test ./...

## バックエンドの結合テスト（PostgreSQL を Docker で起動して実行）
test-integration: db-test-up
	cd backend && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -tags=integration -count=1 ./internal/integrationtest/...

db-test-up:
	docker compose --profile test up -d --wait db-test

db-test-down:
	docker compose --profile test rm -sf db-test

## フロントエンドの型チェックとビルド
test-frontend:
	cd frontend && npm ci && npm run build
