# backend

Smash-hub のバックエンドです。対戦募集・結果報告・レーティング・アカウント管理・インスタンス間連携（連合）を担います。

- 言語: **Go**、DB: **PostgreSQL**（[ADR 0005](../docs/adr/0005-tech-stack.md)）
- 構成: クリーンアーキテクチャ + ドメイン駆動設計のモジュラーモノリス（[バックエンド設計](../docs/architecture/backend.md)）

## 構成

```
cmd/server/           エントリーポイント
internal/
├── modules/          境界づけられたコンテキストごとのモジュール
│   ├── account/      アカウント・外部認証との紐づけ・セッション
│   ├── matchmaking/  対戦相手を探す（自動マッチング）
│   ├── match/        対戦・結果報告・中止・期限切れ
│   ├── rating/       レーティング算出（スマメイト式）・ランキング
│   └── federation/   インスタンス間連携（未実装）
│       （各モジュール: domain/ → application/ → adapter/）
├── shared/           共有カーネル
├── platform/         フレームワーク & ドライバー（DB・HTTP・外部認証・暗号・設定）
├── bootstrap/        依存の組み立て（モジュール間の port を接続する）
└── integrationtest/  PostgreSQL と結合したテスト
```

依存は `adapter → application → domain` の内向きのみです。`domain` はフレームワーク・DB・HTTP に依存しません。

## 実装状況

| 機能 | 状態 |
|---|---|
| 開発用ログイン（`DEV_LOGIN_ENABLED=true` のときだけ有効） | ✅ |
| Discord ログイン（OAuth 2.0 + PKCE、`identify` のみ、識別子ハッシュのみ保存） | ✅（`DISCORD_*` を設定したときだけ有効） |
| アカウント登録・プロフィール編集 | ✅ |
| 対戦相手を探す（レート差の条件、「制限なし」の拡大、離脱の検知） | ✅ |
| 対戦ルーム（部屋 ID の共有・共通パスワード） | ✅ |
| 両者による結果報告・食い違い・中止・24時間での期限切れ | ✅ |
| スマメイト式によるレート算出・ランキング（20戦以上で掲載） | ✅ |
| 戦績（最近の対戦） | ✅ |
| 画面（frontend/） | 動作検証用の仮 UI。正式なデザインは [#10](https://github.com/tacky0612/smash-hub/issues/10) |
| Google ログイン・複数プロバイダーの紐づけ | 未実装 |
| 拒否ステージ・ゲームごとの記録・スイッチ・ブロック・連戦/接続の条件・配信設定 | 未実装 |
| 対戦マナーの目安・通報・モデレーション | 未実装 |
| 鍵ペア・署名付きの対戦記録・追記専用ログ・連合 | 未実装 |
| レート制限・アクセスログの保持期間 | 未実装 |

## 開発

リポジトリのルートで実行します。

```bash
make up                # Docker Compose で全体を起動（http://localhost:8080）
make test-unit         # 単体テスト（DB 不要）
make test-integration  # 結合テスト（PostgreSQL を Docker で起動して実行）
```

結合テストは build tag `integration` を付けたテストで、`TEST_DATABASE_URL` の DB を毎回空にして使います。本番や開発の DB を指定しないでください。

```bash
docker compose --profile test up -d --wait db-test
TEST_DATABASE_URL=postgres://test:test@localhost:55432/smashhub_test?sslmode=disable \
  go test -tags=integration -count=1 ./internal/integrationtest/...
```

## 設定（環境変数）

| 変数 | 必須 | 説明 |
|---|---|---|
| `DATABASE_URL` | ✓ | PostgreSQL の接続先 |
| `IDENTIFIER_PEPPER` | ✓ | 識別子ハッシュ用の秘密値（32バイト以上）。**DB とは別に保管・バックアップする**（[データ保護](../docs/security/data-protection.md)） |
| `HTTP_ADDR` | | 待ち受けるアドレス（既定: `:8081`） |
| `APP_ENV` | | `production` では開発用ログインを有効にできない |
| `PUBLIC_BASE_URL` | | フロントエンドの URL（OAuth のリダイレクト先。既定: `http://localhost:8080`） |
| `SOURCE_CODE_URL` | | 画面に表示するソースコードの URL（AGPL-3.0 第13条。改変した場合は改変版を指定する） |
| `COOKIE_SECURE` | | Cookie に Secure を付けるか（既定: `true`。ローカルの HTTP では `false`） |
| `DEV_LOGIN_ENABLED` | | 開発用ログインを有効にするか（既定: `false`） |
| `DISCORD_CLIENT_ID` / `DISCORD_CLIENT_SECRET` / `DISCORD_REDIRECT_URL` | | Discord ログイン。リダイレクト URL は `<PUBLIC_BASE_URL>/api/auth/discord/callback` |
| `SEASON_ID` / `SEASON_START` | | 現在のシーズン（既定: `S1` / `2026-01-01T00:00:00+09:00`） |
| `RANKING_MIN_MATCHES` | | ランキング掲載に必要な対戦数（既定: 20。ローカル環境では 1） |
