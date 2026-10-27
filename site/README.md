# site（SmashHubプロトコルの紹介サイト）

SmashHubプロトコルを広めるための静的サイトです。GitHub Pages で公開し、`main` への push で自動的にデプロイします（[.github/workflows/site.yml](../.github/workflows/site.yml)）。

| ページ | 内容 | 主な元のドキュメント |
|---|---|---|
| トップ（`#/`） | 解決したい課題・全体像・設計原則 | [ビジョン](../docs/vision.md) |
| 仕組み（`#/how-it-works`） | 署名付きログ・対戦記録・pull 同期・SmashHubレート・インスタンスをまたぐ対戦 | [SmashHubプロトコル](../docs/architecture/federation.md) |
| 開発者向け（`#/developers`） | API・イベントの形式・検証の手順・実装の例 | [SmashHubプロトコル](../docs/architecture/federation.md)、[既存の対戦サイトのための入門](../docs/architecture/federation-for-existing-sites.md) |
| 既存サイトへの導入（`#/adoption`） | 得られるもの・共有するもの・3段階の導入・参加の条件・FAQ | [既存の対戦サイトのための入門](../docs/architecture/federation-for-existing-sites.md)、[運用者の約束事](../docs/operations/server-covenant.md) |
| ポリシー（`#/policy`） | 商標・非営利・個人情報・対戦記録・ライセンス | [運営ポリシー](../docs/policy.md)、[データ保護](../docs/security/data-protection.md) |

**正式な内容はリポジトリのドキュメントです。** サイトはその要約なので、元のドキュメントを変えたらサイトも揃えてください。詳しい内容はサイトに書き写さず、GitHub のドキュメントへリンクします（リンク先は [src/content/links.ts](src/content/links.ts)）。

## 開発

```bash
cd site
npm ci
npm run dev       # http://localhost:5173
npm run build     # 型チェックとビルド（make test-site と同じ）
npm run preview   # ビルドしたものを確認する
```

## 構成

```
src/
├── main.tsx / App.tsx      エントリーポイントとルーティング（HashRouter）
├── routes.ts               ヘッダーに並べるページ
├── index.css               Tailwind CSS の読み込みとテーマ
├── content/links.ts        リポジトリのドキュメントへのリンク
├── components/
│   ├── Layout.tsx          ヘッダー（GitHub へのリンク・スマートフォン用メニュー）とフッター
│   ├── ui.tsx              見出し・セクション・カード・注記・コード・表などの部品
│   ├── IconBadge.tsx       アイコンの枠
│   └── diagrams/           図
│       ├── kit.tsx                 SVG の部品（箱・矢印・領域）と PC/スマートフォンの切り替え
│       ├── ProtocolDiagrams.tsx    関係図・フロー図（PC 用とスマートフォン用の2つの構成）
│       ├── SequenceDiagram.tsx     データから描くシーケンス図（スマートフォンでは縦の時系列）
│       ├── StepFlow.tsx            手順の図
│       └── AdoptionStages.tsx      導入の3段階
└── pages/                  各ページ
```

## 方針

- **技術**: React（コンポーネント）・TypeScript・Vite・[Tailwind CSS](https://tailwindcss.com/)（CSS フレームワーク）・[lucide-react](https://lucide.dev/)（アイコン）。いずれも MIT / ISC ライセンスで、AGPL-3.0 と互換です。
- **公式の素材を使わない**: 任天堂の画像・ロゴ・アイコン・フォントを使わず、公式の意匠に寄せません（[運営ポリシー](../docs/policy.md)）。図はテキストと図形だけで描きます。
- **絵文字を使わない**: アイコンは lucide-react を `IconBadge` などの部品で揃えて使います。
- **外部リソースを読み込まない**: フォント・スクリプト・解析タグを外部から読み込みません（システムフォントのみ）。
- **レスポンシブ**: スマートフォン（幅 390px 前後）と PC の両方で最適な構成にします。横に広い図は、`Responsive` で PC 用とスマートフォン用（縦に並べる構成）を切り替えます。図の内容を変えるときは両方を揃えて直してください。表はスマートフォンでカード表示に切り替わります。
- **配色**: 白地にグレー（slate）の濃淡、差し色は赤（`red-600`）1色です。図のトーンは、主役（インスタンス・記録）を赤、補助（既存サイト・記録 ID）をチャコール、閉鎖などを灰色で表します（`components/diagrams/kit.tsx`）。緑・青などの色は増やさず、注意の囲みだけアンバーを使います。ダークモードには対応しません（背景は常に白）。
- **ルーティング**: GitHub Pages はサーバー側でパスを書き換えられないため、`HashRouter`（`/#/how-it-works`）を使います。ビルドは相対パス（`base: "./"`）なので、リポジトリ名やフォークに依存しません。

## デプロイ

`main` に `site/` の変更が push されると、GitHub Actions がビルドして GitHub Pages に公開します。PR ではビルド（型チェックを含む）だけを行います。

初回だけ、リポジトリの **Settings → Pages → Build and deployment → Source** を **GitHub Actions** にしてください。公開先は `https://<owner>.github.io/<repo>/` です。
