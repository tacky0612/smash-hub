import { useState, type FormEvent } from "react";
import { api } from "../api";
import { useApp } from "../context";
import { FighterSelect } from "./FighterSelect";

export function RegisterPage() {
  const { refreshMe } = useApp();
  const [displayName, setDisplayName] = useState("");
  const [mainFighter, setMainFighter] = useState("");
  const [agreed, setAgreed] = useState(false);
  const [error, setError] = useState("");

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      await api.register(displayName, mainFighter);
      await refreshMe();
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <section className="card narrow">
      <h1>アカウント登録</h1>
      <form onSubmit={submit}>
        <label>
          表示名（1〜20文字）
          <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} maxLength={20} required />
        </label>
        <p className="muted small">表示名は公開されます。本名や連絡先は書かないでください。</p>
        <FighterSelect label="メインファイター" value={mainFighter} onChange={setMainFighter} />
        <label className="checkbox">
          <input type="checkbox" checked={agreed} onChange={(e) => setAgreed(e.target.checked)} />
          確定した対戦記録が公開され、連携する他のサーバーにも複製されることに同意します
        </label>
        {error && <p className="error">{error}</p>}
        <button type="submit" className="button primary block" disabled={!agreed}>
          登録する
        </button>
      </form>
    </section>
  );
}
