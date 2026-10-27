import type { ReactNode } from "react";
import { Link, Navigate, NavLink, Route, Routes } from "react-router";
import { useApp } from "./context";
import { AdminPage } from "./pages/AdminPage";
import { HomePage } from "./pages/HomePage";
import { LoginPage } from "./pages/LoginPage";
import { MatchPage } from "./pages/MatchPage";
import { PlayerPage } from "./pages/PlayerPage";
import { RankingPage } from "./pages/RankingPage";
import { RegisterPage } from "./pages/RegisterPage";
import { SettingsPage } from "./pages/SettingsPage";
import { TermsConsentPage, TermsPage } from "./pages/TermsPage";

export function App() {
  const { me } = useApp();
  if (me === null) return <p className="loading">読み込み中…</p>;
  return (
    <div className="layout">
      <Header />
      <main className="main">
        <Routes>
          <Route path="/login" element={me.loggedIn ? <Navigate to="/" replace /> : <LoginPage />} />
          <Route path="/register" element={<RequireLogin registered={false}><RegisterPage /></RequireLogin>} />
          <Route path="/" element={<RequireLogin><HomePage /></RequireLogin>} />
          <Route path="/matches/:id" element={<RequireLogin><MatchPage /></RequireLogin>} />
          <Route path="/players/:id" element={<RequireLogin><PlayerPage /></RequireLogin>} />
          <Route path="/settings" element={<RequireLogin><SettingsPage /></RequireLogin>} />
          <Route path="/ranking" element={<RankingPage />} />
          <Route path="/terms" element={<TermsPage />} />
          <Route path="/admin" element={<RequireLogin><AdminPage /></RequireLogin>} />
          <Route path="*" element={<p>ページが見つかりません。</p>} />
        </Routes>
      </main>
      <Footer />
    </div>
  );
}

/** ログイン（と、registered が true なら登録）を必須にする。 */
function RequireLogin({ children, registered = true }: { children: ReactNode; registered?: boolean }) {
  const { me } = useApp();
  if (!me?.loggedIn) return <Navigate to="/login" replace />;
  if (registered && !me.registered) return <Navigate to="/register" replace />;
  // 利用規約が更新されていたら、同意してもらうまで他の画面を出さない
  if (registered && me.registered && !me.termsAccepted) return <TermsConsentPage />;
  if (!registered && me.registered) return <Navigate to="/" replace />;
  return <>{children}</>;
}

function Header() {
  const { me, instance, moderation } = useApp();
  const registered = me?.loggedIn && me.registered;
  return (
    <header className="header">
      <Link to="/" className="brand">
        SmashHub
        {instance && instance.name !== "SmashHub" && <span className="instance-name">{instance.name}</span>}
      </Link>
      <nav className="nav">
        {registered && <NavLink to="/">対戦</NavLink>}
        <NavLink to="/ranking">ランキング</NavLink>
        {registered && <NavLink to={`/players/${me.account.id}`}>戦績</NavLink>}
        {registered && <NavLink to="/settings">設定</NavLink>}
        {registered && moderation?.admin && <NavLink to="/admin">管理</NavLink>}
      </nav>
    </header>
  );
}

function Footer() {
  const { instance } = useApp();
  return (
    <footer className="footer">
      <p>
        本サービスは有志による非公式のファンサイトであり、任天堂株式会社とは一切関係ありません。
        「大乱闘スマッシュブラザーズ」は任天堂株式会社の登録商標です。
      </p>
      <p>
        <Link to="/terms">利用規約</Link>
        {instance?.contact && <> ／ 問い合わせ: {instance.contact}</>}
      </p>
      <p>
        {/* AGPL-3.0 第13条: 利用者にソースコードを提示する */}
        <a href={instance?.sourceCodeUrl ?? "https://github.com/tacky0612/smash-hub"} rel="noreferrer">
          ソースコード（AGPL-3.0）
        </a>
      </p>
    </footer>
  );
}
