.PHONY: up down logs test test-unit test-integration test-e2e test-frontend db-test-up db-test-down

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
test: test-unit test-integration test-frontend test-e2e

## バックエンドの単体テスト（DB 不要）
test-unit:
	cd backend && go test ./...

## バックエンドの結合テスト（PostgreSQL を Docker で起動して実行）
test-integration: db-test-up
	cd backend && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -tags=integration -count=1 ./internal/integrationtest/...

## 2つのインスタンスを Docker で起動し、連合プロトコルでの対戦の流れを検証する（E2E テスト）
test-e2e: up
	cd backend && INSTANCE1_URL="$(INSTANCE1_URL)" INSTANCE2_URL="$(INSTANCE2_URL)" go test -tags=e2e -count=1 ./e2e/...

db-test-up:
	docker compose --profile test up -d --wait db-test

db-test-down:
	docker compose --profile test rm -sf db-test

## フロントエンドの型チェックとビルド
test-frontend:
	cd frontend && npm ci && npm run build
