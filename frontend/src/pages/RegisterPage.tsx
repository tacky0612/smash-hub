import { useEffect, useState, type FormEvent } from "react";
import { api, type Terms } from "../api";
import { useApp } from "../context";
import { FighterSelect } from "./FighterSelect";
import { Markdown } from "./Markdown";
import { PolicyLink } from "./TermsPage";

export function RegisterPage() {
  const { refreshMe } = useApp();
  const [terms, setTerms] = useState<Terms | null>(null);
  const [displayName, setDisplayName] = useState("");
  const [mainFighter, setMainFighter] = useState("");
  const [agreed, setAgreed] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    void api.terms().then(setTerms);
  }, []);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!terms) return;
    setError("");
    try {
      await api.register(displayName, mainFighter, terms.version);
      await refreshMe();
    } catch (err) {
      setError((err as Error).message);
      // 利用規約が更新されていた場合に備えて読み直す
      setTerms(await api.terms());
      setAgreed(false);
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
        <div className="terms-box">{terms ? <Markdown text={terms.text} /> : <p className="muted">読み込み中…</p>}</div>
        <PolicyLink />
        <label className="checkbox">
          <input type="checkbox" checked={agreed} onChange={(e) => setAgreed(e.target.checked)} />
          利用規約と運営ポリシーに同意します（確定した対戦記録が公開され、連携する他のサーバーにも複製されることを含みます）
        </label>
        {error && <p className="error">{error}</p>}
        <button type="submit" className="button primary block" disabled={!agreed || !terms}>
          登録する
        </button>
      </form>
    </section>
  );
}
