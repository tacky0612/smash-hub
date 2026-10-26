# AGENTS.md

AI コーディングエージェント（Claude Code・Codex・Copilot 等）向けのガイドです。人間のコントリビューター向けには [CONTRIBUTING.md](CONTRIBUTING.md) があります。両者の内容が食い違う場合は、このファイルを直して揃えてください。

## プロジェクト概要

Smash-hub は、大乱闘スマッシュブラザーズ SPECIAL の対戦マッチング・結果報告・レーティングを提供する **分散（連合）型・非営利・OSS** のファンサイトです。複数の独立した運営者のインスタンスが署名付きの対戦記録と対戦募集を共有し合い、1つのインスタンスが閉鎖しても記録とサービスが継続すること、サイトごとにレートが分断されない（連合全体で1つのレート）こと、利用者の少ないインスタンスでも別のインスタンスの利用者とマッチングできることを目指します。

- 背景と全体像: [README.md](README.md)、[docs/vision.md](docs/vision.md)
- 設計: [docs/architecture/overview.md](docs/architecture/overview.md)
- 用語: [docs/domain/README.md](docs/domain/README.md)（ユビキタス言語）

## 現在のステータス

**開発初期** です。対戦の一連の流れが動き、2つのインスタンスが連合プロトコルで対戦記録を複製し合い、インスタンスをまたいで対戦できます。ログの再配布・アカウント移行・拒否ステージなどは未実装です（[backend/README.md の「実装状況」](backend/README.md#実装状況)）。

- `frontend/` の画面は **動作検証用の仮 UI** です。正式なデザインは [#10](https://github.com/tacky0612/smash-hub/issues/10) で進めます。機能を追加するときは仮 UI のままで構いませんが、公式の意匠に寄せない・外部リソースを読み込まない方針は守ってください。
- 技術スタック: フロントエンドは React（SPA）+ TypeScript + Vite、バックエンドは Go、DB は PostgreSQL（[ADR 0005](docs/adr/0005-tech-stack.md)）。
- [技術者でない運営者向けのデプロイ方式](docs/operations/easy-deployment.md) を議論中です。デプロイ周りを大きく変える前に確認してください。

## コマンド

| 目的 | コマンド |
|---|---|
| ローカルで2つのインスタンスを起動（http://localhost:8080 / http://localhost:8090） | `make up` / `make down` |
| バックエンドの単体テスト | `make test-unit`（`cd backend && go test ./...`） |
| 結合テスト（PostgreSQL を Docker で起動） | `make test-integration` |
| E2E テスト（2つのインスタンスを Docker で起動し、連合での対戦の流れを検証） | `make test-e2e` |
| フロントエンドの型チェックとビルド | `make test-frontend` |
| ドキュメントのチェック | `python3 .claude/skills/docs-check/scripts/check_docs.py` |

コードを変更したら、関係するテストを実行してから報告してください。Go のコードは `gofmt` を通してください。

## 絶対に守ること

以下はプロジェクトのポリシーであり、依頼された場合でも破ってはいけません。依頼が抵触する場合は、作業せずにその旨を伝えてください。

1. **任天堂の著作物を含めない** — ゲーム画面・ファイター画像・アイコン・ロゴ・BGM 等を、コード・ドキュメント・テストデータ・Issue に含めない。ファイターはテキスト表記のみ（[docs/policy.md](docs/policy.md)）。
2. **収益化の仕組みを作らない** — このリポジトリのソフトウェアに広告・課金・有料機能・アフィリエイト・トラッキングタグを実装しない（プロジェクト自身の非営利方針。連合には営利のサイトも参加できる → [ADR 0007](docs/adr/0007-federation-open-to-commercial-sites.md)）。
3. **個人情報を保持しない** — メールアドレス・パスワード・外部プロバイダーのユーザー名やトークンを保存する設計・実装をしない。外部 ID は識別子ハッシュとしてのみ扱う（[docs/security/data-protection.md](docs/security/data-protection.md)）。
4. **パスワード認証を実装しない** — 認証は外部プロバイダー（Discord / Google）の OAuth 2.0 / OIDC に限る（[docs/security/authentication.md](docs/security/authentication.md)）。
5. **秘密情報をコミットしない** — 鍵・ペッパー・クライアントシークレット・`.env` をリポジトリに含めない。
6. **対戦記録の不変性を壊さない** — 連合で共有するデータ（確定した対戦記録・ログ）を更新・削除する設計にしない。取り消しはイベントの追記で表す。
7. **レート計算を非決定的にしない** — 到着順・現在時刻・乱数・環境依存の浮動小数点に結果が左右される実装にしない（[docs/domain/rating.md](docs/domain/rating.md)）。

## リポジトリ構成

```
README.md / CONTRIBUTING.md / CODE_OF_CONDUCT.md / SECURITY.md
AGENTS.md / CLAUDE.md     エージェント向けガイド
.claude/skills/           Claude Code のスキル
.github/                  Issue / PR テンプレート、CODEOWNERS、CI（docs-check）
docs/                     設計・ポリシー・ロードマップ（日本語）
  architecture/           全体像・連合プロトコル・バックエンド・フロントエンド
  domain/                 ユビキタス言語・コンテキスト・レーティング
  security/               セキュリティポリシー・認証・データ保護・脅威モデル・インシデント対応
  operations/             運用者向け（約束事・デプロイ方式の議論）
  adr/                    設計判断の記録
frontend/                 Web UI（React SPA）
backend/                  クリーンアーキテクチャ + DDD のモジュラーモノリス（Go）
compose.yaml / Makefile   ローカル環境と起動・テストのコマンド
```

## ドキュメントの規約

- **日本語で書く。** コード識別子・技術用語は原語のまま。
- **用語は [ユビキタス言語](docs/domain/README.md) に従う。** 新しい概念を導入したら用語表に追加する。
- 設計ドキュメントの冒頭に `> ステータス: **Draft**` 等のステータスを書く。
- 決まっていないことは各ドキュメント末尾の「未解決の論点」にチェックリストで書く。
- 図は Mermaid で書く（GitHub で表示できるため）。
- リンクは相対パスで書く。
- **重要な設計判断は ADR にする。** [docs/adr/0000-template.md](docs/adr/0000-template.md) をコピーし連番を振り、[docs/adr/README.md](docs/adr/README.md) の一覧に追加する。既存の ADR は書き換えず、新しい ADR で置き換える（`Superseded by NNNN`）。
- **ドキュメントを追加・移動・削除したら** [docs/README.md](docs/README.md) の目次を更新する。
- 1つの事実は1か所に書き、他の場所からはリンクする。同じ内容を複数箇所に書いた場合は、すべてを揃えて更新する（例: 個人情報の方針は policy.md・security/・server-covenant.md・README.md に現れる）。
- ドキュメントを変更したら **`docs-check` スキル** で整合性とリンク切れを確認する。スクリプトだけなら `python3 .claude/skills/docs-check/scripts/check_docs.py` で実行できる。

## バックエンドの規約

詳細は [docs/architecture/backend.md](docs/architecture/backend.md)。

- モジュール: `account` / `matchmaking` / `match` / `rating` / `federation`（境界づけられたコンテキストと一致させる）
- 各モジュールは `domain/` → `application/` → `adapter/` の3層。依存は内側（`domain`）へのみ向ける。
- `domain` はフレームワーク・DB・HTTP・時刻・乱数に直接依存しない（`shared/domain` のインターフェース経由）。
- モジュール間は他モジュールの `domain` を直接参照せず、ドメインイベントか `application/port` 経由で連携する。
- Go のコードは `backend/internal/` に置く。モジュールは `internal/modules/<module>/{domain,application,adapter}`。
- 他モジュールの機能が必要なときは、使う側の `application` に port（インターフェース）を定義し、`internal/bootstrap` で接続する。
- DB を使う処理を追加したら `internal/integrationtest/` に結合テストを追加する。インスタンス間の通信に関わる変更は `e2e/` の E2E テストも追加・更新する。
- 新しいモジュールやディレクトリを追加したら、backend.md と backend/README.md の構成図も更新する。

## ライセンス

リポジトリ全体が [AGPL-3.0](LICENSE) です（[ADR 0003](docs/adr/0003-license-agpl-3.0.md)）。依存ライブラリ・素材を追加する場合は AGPL-3.0 と互換性のあるライセンスか確認し、PR に記載してください。

## Git

- `main` へ直接コミット・push しない。作業ブランチを切り、PR を出す。
- push・PR 作成・Issue 作成など外部に公開される操作は、ユーザーの確認を得てから行う（リポジトリは公開されています）。
- PR では [PR テンプレート](.github/PULL_REQUEST_TEMPLATE.md) のチェック項目を確認する。
