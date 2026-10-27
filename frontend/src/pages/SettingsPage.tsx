import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { api, CATEGORY_LABELS, SANCTION_LABELS } from "../api";
import { useApp } from "../context";
import { FighterSelect } from "./FighterSelect";

export function SettingsPage() {
  const { me, moderation, refreshMe } = useApp();
  const navigate = useNavigate();
  const account = me?.loggedIn && me.registered ? me.account : null;
  const [displayName, setDisplayName] = useState(account?.displayName ?? "");
  const [mainFighter, setMainFighter] = useState(account?.mainFighter ?? "");
  const [message, setMessage] = useState("");

  const save = async (e: FormEvent) => {
    e.preventDefault();
    try {
      await api.updateProfile(displayName, mainFighter);
      await refreshMe();
      setMessage("保存しました");
    } catch (err) {
      setMessage((err as Error).message);
    }
  };

  const logout = async () => {
    await api.logout();
    await refreshMe();
    navigate("/login");
  };

  return (
    <>
      <section className="card narrow">
        <h1>プロフィール</h1>
        <form onSubmit={save}>
          <label>
            表示名（1〜20文字）
            <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} maxLength={20} required />
          </label>
          <FighterSelect label="メインファイター" value={mainFighter} onChange={setMainFighter} />
          {message && <p className="muted">{message}</p>}
          <button type="submit" className="button primary block">
            保存
          </button>
        </form>
      </section>
      <section className="card narrow">
        <h2>アカウント ID</h2>
        <p className="mono small break">{account?.id}</p>
        <p className="muted small">
          連合全体であなたを表す ID です（公開されます）。インスタンスの運営者は、この ID を設定ファイルの ADMIN_ACCOUNT_IDS
          に書くと管理者になります。
        </p>
      </section>
      {moderation && moderation.sanctions.length > 0 && (
        <section className="card narrow">
          <h2>運営者からの処分</h2>
          <ul className="history">
            {moderation.sanctions.map((s) => (
              <li key={s.id}>
                <span>{SANCTION_LABELS[s.kind]}</span>
                <span>
                  {CATEGORY_LABELS[s.reason] ?? s.reason}
                  {s.kind === "red" && (
                    <span className="muted small">
                      {" "}
                      {s.liftedAt ? "解除済み" : s.endsAt ? `${new Date(s.endsAt).toLocaleString("ja-JP")} まで` : "無期限"}
                    </span>
                  )}
                </span>
                <time className="muted small">{new Date(s.createdAt).toLocaleString("ja-JP")}</time>
              </li>
            ))}
          </ul>
          <p className="muted small">対応に納得できない場合は、利用規約の問い合わせ先に申し立ててください。</p>
        </section>
      )}
      <section className="card narrow">
        <button className="button block" onClick={logout}>
          ログアウト
        </button>
      </section>
    </>
  );
}
