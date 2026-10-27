# コントリビューションガイド

Smash-hub に興味を持ってくれてありがとうございます！
このプロジェクトは「運営者・開発者に負担が集中しない」ことを目指しています。そのため、**参加してくれる人が増えること自体がプロジェクトの目的の一部** です。

## 参加の方法

コードを書かなくても参加できます。

| やりたいこと | 方法 |
|---|---|
| 設計について意見したい | Discussion や設計ドキュメントへの Issue / PR |
| 技術選定に意見したい | [技術選定](docs/tech-selection.md) について Discussion で議論 |
| 実装したい | `good first issue` / `help wanted` ラベルの Issue から |
| UI デザインをしたい | 現在の画面は動作検証用の仮 UI です。[#10](https://github.com/tacky0612/smash-hub/issues/10) で正式なデザイン（アイコン・Favicon・画面構成）を進めています |
| サーバーを運用したい | [ロードマップ](docs/roadmap.md) Phase 2 のミラー運用から参加できる予定です |
| ドキュメントを直したい | 誤字・分かりにくい箇所の PR を歓迎します |
| バグを報告したい | Issue テンプレートに沿って報告 |

## 進め方

1. 大きな変更の前に、Issue または Discussion で方針を相談してください。
2. リポジトリを fork し、ブランチを作成して作業します。
3. PR を作成し、テンプレートに沿って説明を書きます。
4. レビューを経てマージされます。

### 設計判断を伴う変更

技術選定・プロトコル・データ形式・ポリシーに関わる変更は、[ADR](docs/adr/README.md) を添えて提案してください。

## 必ず守ってほしいこと

- **任天堂の著作物を含めないでください。** ゲーム画面・キャラクター画像・公式ロゴ・アイコン等を Issue・PR・コードに含めないでください。詳しくは [運営ポリシー](docs/policy.md) を参照してください。
- **素材を追加する場合は出典とライセンスを明記してください。**
- **脆弱性は公開の Issue に書かず、[SECURITY.md](SECURITY.md) の手順で報告してください。**
- **個人情報を収集・保存する機能を追加しないでください。** 方針は [データ保護](docs/security/data-protection.md) を参照してください。
- **[行動規範](CODE_OF_CONDUCT.md) に従ってください。**

## ドキュメントのチェック

ドキュメントを変更したら、リンク切れや目次漏れがないか確認してください。

```bash
python3 .claude/skills/docs-check/scripts/check_docs.py
```

このチェックは PR ごとに GitHub Actions（`.github/workflows/docs-check.yml`）でも自動実行されます。

AI エージェントを使って開発する場合は [AGENTS.md](AGENTS.md) を参照してください。

## 開発環境

必要なもの: [Docker](https://www.docker.com/)（Compose v2）。バックエンドを直接触る場合は Go 1.26 以上、フロントエンドを直接触る場合は Node.js 24 以上。

```bash
make up                # 2つのインスタンスを起動（Instance1: http://localhost:8080 / Instance2: http://localhost:8090）
make down              # 停止
make logs              # ログを見る
make test-unit         # バックエンドの単体テスト
make test-integration  # バックエンドの結合テスト（PostgreSQL を Docker で起動）
make test-e2e          # E2E テスト（2つのインスタンスで、連合での対戦の流れを検証）
make test-frontend     # フロントエンドの型チェックとビルド
make test-site         # 紹介サイト（GitHub Pages）の型チェックとビルド
```

- 2つのインスタンスは互いを連合先にしています。Instance1 と Instance2 にそれぞれ別の利用者でログインすると、インスタンスをまたいだ対戦と記録の複製を試せます。
- データベースの形式を変えた後などに初期化したい場合は `docker compose down -v` で DB を消してから `make up` してください。

- ローカル環境では **開発用ログイン**（名前を入れるだけ）が有効です。ブラウザのシークレットウィンドウや別のブラウザを使うと、2人目の利用者として対戦を試せます。
- Discord ログインを試す場合は、Discord Developer Portal でアプリを作り、`DISCORD_CLIENT_ID`・`DISCORD_CLIENT_SECRET`・`DISCORD_REDIRECT_URL`（`http://localhost:8080/api/auth/discord/callback`）を環境変数で渡して `make up` してください。
- 個別の詳細は [backend/README.md](backend/README.md)・[frontend/README.md](frontend/README.md) を参照してください。

## 開発のルール

- バックエンドは [クリーンアーキテクチャ + DDD](docs/architecture/backend.md) の依存ルールに従ってください。
- Go のコードは `gofmt` を通してください。DB を使う処理には結合テストを追加してください。
- PR では CI（`ci`・`docs-check`）が通ることを確認してください。
- コミットメッセージは日本語で、変更の内容が分かるように書いてください。

## ライセンス

Smash-hub は [AGPL-3.0](LICENSE) で公開しています。コントリビューションは AGPL-3.0 で提供されたものとみなします。

依存ライブラリや素材を追加する場合は、AGPL-3.0 と互換性のあるライセンスであることを確認してください。
