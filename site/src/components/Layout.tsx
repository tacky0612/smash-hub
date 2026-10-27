import { useEffect, useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router";
import { NAV } from "../routes";
import { DOCS, REPO } from "../content/links";

/** 全ページ共通の枠（ヘッダー・フッター）。 */
export function Layout() {
  const { pathname } = useLocation();
  // ページを移動したら先頭に戻す（HashRouter ではスクロール位置が残るため）
  useEffect(() => window.scrollTo(0, 0), [pathname]);
  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1">
        <Outlet />
      </main>
      <Footer />
    </div>
  );
}

function Header() {
  const [open, setOpen] = useState(false);
  const { pathname } = useLocation();
  useEffect(() => setOpen(false), [pathname]);

  const linkClass = ({ isActive }: { isActive: boolean }) =>
    `rounded-md px-3 py-2 text-sm font-medium transition-colors ${
      isActive
        ? "bg-indigo-50 text-indigo-700 dark:bg-indigo-950 dark:text-indigo-300"
        : "text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-white"
    }`;

  return (
    <header className="sticky top-0 z-30 border-b border-slate-200 bg-white/90 backdrop-blur dark:border-slate-800 dark:bg-slate-950/90">
      <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3 sm:px-6">
        <NavLink to="/" className="flex items-center gap-2 font-bold text-slate-900 dark:text-white">
          <Mark />
          <span>Smash-hub</span>
          <span className="hidden rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-500 sm:inline dark:bg-slate-800 dark:text-slate-400">
            連合プロトコル
          </span>
        </NavLink>

        <nav className="hidden items-center gap-1 lg:flex" aria-label="メイン">
          {NAV.map((item) => (
            <NavLink key={item.path} to={item.path} end={item.path === "/"} className={linkClass}>
              {item.label}
            </NavLink>
          ))}
          <GitHubLink />
        </nav>

        <div className="flex items-center gap-2 lg:hidden">
          <GitHubLink compact />
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            aria-expanded={open}
            aria-controls="mobile-nav"
            className="rounded-md p-2 text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            <span className="sr-only">メニュー</span>
            <svg viewBox="0 0 24 24" className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth={2} aria-hidden>
              {open ? <path d="M6 6l12 12M18 6L6 18" /> : <path d="M4 7h16M4 12h16M4 17h16" />}
            </svg>
          </button>
        </div>
      </div>
      {open && (
        <nav id="mobile-nav" className="border-t border-slate-200 px-4 pb-4 pt-2 lg:hidden dark:border-slate-800" aria-label="メイン">
          <div className="flex flex-col gap-1">
            {NAV.map((item) => (
              <NavLink key={item.path} to={item.path} end={item.path === "/"} className={linkClass}>
                {item.label}
              </NavLink>
            ))}
          </div>
        </nav>
      )}
    </header>
  );
}

/** GitHub のリポジトリへのリンク（外部サイトなので新しいタブで開く）。 */
function GitHubLink({ compact = false }: { compact?: boolean }) {
  return (
    <a
      href={REPO}
      target="_blank"
      rel="noopener noreferrer"
      className="ml-1 inline-flex items-center gap-2 rounded-md bg-slate-900 px-3 py-2 text-sm font-medium text-white hover:bg-slate-700 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-200"
    >
      <svg viewBox="0 0 16 16" className="h-4 w-4" fill="currentColor" aria-hidden>
        <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
      </svg>
      <span className={compact ? "sr-only" : undefined}>GitHub</span>
    </a>
  );
}

/** プロジェクトの目印（公式の意匠に寄せない、ノードとつながりの抽象図形）。 */
function Mark() {
  return (
    <svg viewBox="0 0 32 32" className="h-7 w-7" aria-hidden>
      <circle cx="8" cy="9" r="4" className="fill-indigo-500" />
      <circle cx="24" cy="9" r="4" className="fill-indigo-500" />
      <circle cx="16" cy="24" r="4" className="fill-amber-500" />
      <path d="M8 9L24 9L16 24Z" className="fill-none stroke-slate-400" strokeWidth="2" />
    </svg>
  );
}

function Footer() {
  return (
    <footer className="border-t border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900">
      <div className="mx-auto grid max-w-6xl gap-6 px-4 py-10 text-sm text-slate-600 sm:px-6 md:grid-cols-3 dark:text-slate-400">
        <div className="md:col-span-2">
          <p className="font-semibold text-slate-900 dark:text-slate-100">Smash-hub</p>
          <p className="mt-2 leading-relaxed">
            本サイトおよび Smash-hub は有志による非公式のファンプロジェクトであり、任天堂株式会社とは一切関係ありません。
            「大乱闘スマッシュブラザーズ」は任天堂株式会社の登録商標です。
          </p>
        </div>
        <ul className="space-y-2">
          <li>
            <a className="prose-link" href={REPO} target="_blank" rel="noopener noreferrer">
              ソースコード（GitHub）
            </a>
          </li>
          <li>
            <a className="prose-link" href={DOCS.license} target="_blank" rel="noopener noreferrer">
              ライセンス（AGPL-3.0）
            </a>
          </li>
          <li>
            <a className="prose-link" href={DOCS.contributing} target="_blank" rel="noopener noreferrer">
              コントリビュートする
            </a>
          </li>
        </ul>
      </div>
    </footer>
  );
}
