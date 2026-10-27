// ヘッダーのナビゲーションに並べるページ。ページを追加したらここと App.tsx の Route に足す。
export const NAV = [
  { path: "/", label: "トップ" },
  { path: "/how-it-works", label: "仕組み" },
  { path: "/developers", label: "開発者向け" },
  { path: "/adoption", label: "既存サイトへの導入" },
  { path: "/policy", label: "ポリシー" },
] as const;
