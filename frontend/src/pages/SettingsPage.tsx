import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { api } from "../api";
import { useApp } from "../context";
import { FighterSelect } from "./FighterSelect";

export function SettingsPage() {
  const { me, refreshMe } = useApp();
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
        <button className="button block" onClick={logout}>
          ログアウト
        </button>
      </section>
    </>
  );
}
