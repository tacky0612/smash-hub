# backend

SmashHub のバックエンドです。対戦募集・結果報告・レーティング・アカウント管理・インスタンス間連携（連合）を担います。

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
│   ├── federation/   連合（署名付き追記専用ログ・pull 同期・検証・インスタンス間の対戦の受け口）
│   └── moderation/   通報・処分（注意・イエローカード・レッドカード）・対戦記録の無効化の指示
│       （各モジュール: domain/ → application/ → adapter/）
├── shared/           共有カーネル
├── platform/         フレームワーク & ドライバー（DB・HTTP・レート制限・アクセスログ・外部認証・暗号・設定・利用規約）
├── bootstrap/        依存の組み立て（モジュール間の port を接続する）
└── integrationtest/  PostgreSQL と結合したテスト
e2e/                  Docker で立てた2つのインスタンスに対する E2E テスト（連合での対戦の流れ）
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
| ゲームごとのファイターの報告（任意。両者の入力が一致したときだけ記録に含める） | ✅ |
| 拒否ステージ・スイッチ・ブロック・連戦/接続の条件・配信設定 | 未実装 |
| 通報（テキストのみ）・管理者による確認・処分（注意・イエローカード・レッドカード）・利用停止中の対戦の禁止・通報の削除（対応完了から 30日） | ✅ |
| 通報への証拠のスクリーンショットの添付・対戦マナーの目安 | 未実装 |
| 利用規約の表示と登録時の同意・更新時の再同意（プロジェクト共通のひな形。`TERMS_FILE` で差し替え） | ✅ |
| アカウントの鍵ペア（did:key の ID、秘密鍵は暗号化して保存）・AccountPublished | ✅ |
| 両者の署名付きの対戦記録・インスタンス署名付きの追記専用ログ（MatchRecorded） | ✅ |
| 連合先のログの pull 同期と検証（署名・連番・ハッシュチェーン・プレイヤー署名） | ✅ |
| レートを連合で共有された対戦記録から計算（取り込みの締め切り 7日） | ✅ |
| インスタンスをまたぐ対戦（募集の配信・申し込み・状態の取得・報告の中継） | ✅ |
| 対戦記録の無効化（MatchVoided。記録のホストだけが対戦から 7日以内に発行し、連合先でもレート計算から除外） | ✅ |
| ログのスナップショットの出力と、スナップショットからのレートの再計算・突き合わせ（`server snapshot` / `server verify-snapshot`） | ✅（HTTP での配布は未実装） |
| ログの再配布・鍵の更新・アカウント移行 | 未実装 |
| レート制限・アクセスログの記録と保持期間（初期値 14日）での削除 | ✅ |

## 開発

リポジトリのルートで実行します。

```bash
make up                # Docker Compose で全体を起動（http://localhost:8080）
make test-unit         # 単体テスト（DB 不要）
make test-integration  # 結合テスト（PostgreSQL を Docker で起動して実行）
make test-e2e          # E2E テスト（2つのインスタンスを Docker で起動し、連合での対戦の流れを検証）
```

`make up` では Instance1（http://localhost:8080）と Instance2（http://localhost:8090）が起動し、互いを連合先にします。E2E テスト（`e2e/`、build tag `e2e`）は2つのインスタンスの HTTP API を操作して、インスタンスをまたぐマッチング・報告の中継・記録の複製・両インスタンスでのレートの一致を検証します。CI でも同じテストを実行します。

ログのスナップショット（[federation.md の 8.3](../docs/architecture/federation.md#83-スナップショット)）は、サーバーと同じバイナリのサブコマンドで出力・検証します。

```bash
go run ./cmd/server snapshot -o snapshot.json   # サーバーと同じ環境変数で DB に接続して書き出す
go run ./cmd/server verify-snapshot snapshot.json  # DB 不要。ログを検証し、レートを再計算して突き合わせる
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
| `CLIENT_IP_HEADER` | | 接続元の IP アドレスを伝えるヘッダー。frontend の nginx の後ろに置く場合は `X-Real-IP`（ローカル環境で設定済み）。空なら接続元のアドレスを使う。**利用者が偽装できるヘッダーなので、バックエンドをプロキシの後ろにだけ置くときに設定する** |
| `ACCESS_LOG_RETENTION_DAYS` | | アクセスログの保持期間（日。既定: 14。[データ保護](../docs/security/data-protection.md)） |
| `DISCORD_CLIENT_ID` / `DISCORD_CLIENT_SECRET` / `DISCORD_REDIRECT_URL` | | Discord ログイン。リダイレクト URL は `<PUBLIC_BASE_URL>/api/auth/discord/callback` |
| `SEASON_ID` / `SEASON_START` | | 現在のシーズン（既定: `S1` / `2026-01-01T00:00:00+09:00`） |
| `RANKING_MIN_MATCHES` | | ランキング掲載に必要な対戦数（既定: 20。ローカル環境では 1） |
| `INSTANCE_URL` | ✓ | インスタンス ID。連合先から到達できるベース URL |
| `INSTANCE_NAME` | | 表示用のインスタンス名 |
| `INSTANCE_PRIVATE_KEY` | ✓ | インスタンスの署名鍵（Ed25519 の 32バイトのシードを base64 にしたもの）。**DB とは別に保管・バックアップする** |
| `ACCOUNT_KEY_SECRET` | ✓ | アカウントの秘密鍵を暗号化する鍵暗号化鍵（32バイト以上）。**DB とは別に保管・バックアップする** |
| `PEERS` | | 連合先（許可リスト）のベース URL をカンマ区切りで |
| `FEDERATION_SYNC_INTERVAL` | | 連合先のログを取りに行く間隔（既定: `5s`） |
| `ADMIN_ACCOUNT_IDS` | | 管理者（通報の確認・処分・無効化ができる）のアカウント ID（`did:key:...`）をカンマ区切りで。アカウント ID は画面の「設定」で確認できる |
| `DEV_ADMIN_NAMES` | | 開発用ログインの名前で管理者を指定する（カンマ区切り）。`DEV_LOGIN_ENABLED=true` のときだけ設定できる（ローカル環境では `admin`・`e2e-admin`） |
| `TERMS_FILE` | | インスタンス独自の利用規約（Markdown）のパス。空ならプロジェクト共通のひな形（`internal/platform/terms/default.md`）。本文の `{{instance_name}}`・`{{contact}}`・`{{policy_url}}`・`{{rules_url}}` は置き換える |
| `OPERATOR_CONTACT` | | 運営者の問い合わせ先（利用規約と画面のフッターに表示する。例: Discord サーバーの招待 URL） |
