# backend

Smash-hub のバックエンドです。対戦募集・結果報告・レーティング・アカウント管理・インスタンス間連携（連合）を担います。

> 言語・フレームワークは **技術選定中** です（[技術選定](../docs/tech-selection.md)）。現在はディレクトリ構成のみ用意しています。

## 構成

クリーンアーキテクチャ + ドメイン駆動設計のモジュラーモノリスです。詳細と依存のルールは [バックエンド設計](../docs/architecture/backend.md) を参照してください。

```
src/
├── modules/          境界づけられたコンテキストごとのモジュール
│   ├── account/      アカウント・鍵・プロフィール
│   ├── matchmaking/  対戦募集・マッチング
│   ├── match/        結果報告・中止・対戦記録
│   ├── rating/       レーティング算出・ランキング
│   └── federation/   インスタンス間連携・ログ複製
│       （各モジュール: domain/ → application/ → adapter/）
├── shared/           共有カーネル
├── platform/         フレームワーク & ドライバー（DB・HTTP・外部認証・暗号・設定・ログ）
└── bootstrap/        依存の組み立てと起動
tests/
├── unit/
├── integration/
└── e2e/
```

依存は `adapter → application → domain` の内向きのみです。`domain` はフレームワーク・DB・HTTP に依存しません。
