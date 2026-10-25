# frontend

Smash-hub のプレイヤー向け Web UI です。React（SPA）+ TypeScript + Vite で作っています（[ADR 0005](../docs/adr/0005-tech-stack.md)）。

> [!IMPORTANT]
> 現在の画面は **動作検証用の仮 UI** です。対戦の一連の流れを確認するための最低限の画面で、デザイン・UX はまだ検討していません（配色・レイアウト・文言も仮）。正式な UI デザイン（サービスアイコン・Favicon・画面構成の見直しなど）は [#10](https://github.com/tacky0612/smash-hub/issues/10) で進めます。

方針・主要画面・構成は [フロントエンド設計](../docs/architecture/frontend.md) を参照してください。

**注意:** ゲーム画面・ファイター画像・公式ロゴ・公式アイコン等の任天堂の著作物は使用しません。ファイターはテキストで表示します（[運営ポリシー](../docs/policy.md)）。

## 開発

```bash
npm ci
npm run dev      # http://localhost:5173（/api は http://localhost:8081 のバックエンドへ転送）
npm run build    # 型チェックとビルド
```

バックエンドもまとめて動かす場合は、リポジトリのルートで `make up` を実行し http://localhost:8080 を開いてください。
`npm run dev` でバックエンドの転送先を変えるときは `API_PROXY_TARGET` を指定します。
