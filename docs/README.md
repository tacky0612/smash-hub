# SmashHub ドキュメント

SmashHub の設計・方針・計画をまとめたドキュメント群です。
開発初期のため内容は随時変わります。疑問や異論は Issue / Discussion で歓迎します。

## はじめに読む

1. [ビジョン](vision.md) — なぜ作るのか、何を作らないのか
2. [運営ポリシー](policy.md) — 商標・素材・非営利・プライバシー
3. [ロードマップ](roadmap.md) — スモールスタートの段階計画

## 設計

| ドキュメント | 内容 |
|---|---|
| [architecture/overview.md](architecture/overview.md) | システム全体像と主要な設計原則 |
| [architecture/federation.md](architecture/federation.md) | サーバー間連携（連合）プロトコル |
| [architecture/federation-walkthrough.md](architecture/federation-walkthrough.md) | 図解: 複数のインスタンスが勝敗記録とレートを共有する仕組み |
| [architecture/federation-for-existing-sites.md](architecture/federation-for-existing-sites.md) | 既存の対戦サイトの開発者向けの SmashHubプロトコル入門（参加のメリットと導入の手順） |
| [architecture/backend.md](architecture/backend.md) | バックエンドのクリーンアーキテクチャ + DDD 構成 |
| [architecture/frontend.md](architecture/frontend.md) | フロントエンドの方針 |

## ドメイン

| ドキュメント | 内容 |
|---|---|
| [domain/README.md](domain/README.md) | ユビキタス言語と境界づけられたコンテキスト |
| [domain/fighters.md](domain/fighters.md) | ファイター ID とファイターの対応表 |
| [domain/match-rules.md](domain/match-rules.md) | レーティング対戦のルール（スマメイトのルールを踏襲） |
| [domain/rating.md](domain/rating.md) | 分散環境でのレーティング算出 |

## 判断・選定

| ドキュメント | 内容 |
|---|---|
| [tech-selection.md](tech-selection.md) | 技術選定の基準と候補 |
| [adr/](adr/README.md) | Architecture Decision Records（設計判断の記録） |

## セキュリティ

| ドキュメント | 内容 |
|---|---|
| [security/README.md](security/README.md) | セキュリティポリシー |
| [security/authentication.md](security/authentication.md) | 外部認証プロバイダーによる認証 |
| [security/data-protection.md](security/data-protection.md) | 個人情報を持たないデータ設計 |
| [security/threat-model.md](security/threat-model.md) | 脅威モデル |
| [security/incident-response.md](security/incident-response.md) | インシデント対応 |

脆弱性の報告方法はリポジトリ直下の [SECURITY.md](../SECURITY.md) を参照してください。

## 運用

| ドキュメント | 内容 |
|---|---|
| [operations/server-covenant.md](operations/server-covenant.md) | 連合に参加するサーバー運用者の約束事 |
| [operations/easy-deployment.md](operations/easy-deployment.md) | 技術者でなくてもインスタンスを立てられるようにするための議論（結論は [ADR 0009](adr/0009-deployment-vps-startup-script.md)） |

## ドキュメントの書き方

- 日本語で書きます（コード識別子・技術用語は原語のまま）。
- 図は GitHub で表示できる [Mermaid](https://mermaid.js.org/) を使います。
- 決まったことは ADR に、まだ決まっていないことは各ドキュメントの「未解決の論点」に書きます。
