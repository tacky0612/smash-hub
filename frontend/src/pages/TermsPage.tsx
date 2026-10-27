import { useEffect, useState, type FormEvent } from "react";
import { api, type Terms } from "../api";
import { useApp } from "../context";
import { Markdown } from "./Markdown";

export function TermsPage() {
  const [terms, setTerms] = useState<Terms | null>(null);
  useEffect(() => {
    void api.terms().then(setTerms);
  }, []);
  if (!terms) return <p className="loading">読み込み中…</p>;
  return (
    <section className="card">
      <Markdown text={terms.text} />
      <PolicyLink />
    </section>
  );
}

/** 利用規約が更新されたときに、登録済みの利用者に同意し直してもらう画面。 */
export function TermsConsentPage() {
  const { refreshMe } = useApp();
  const [terms, setTerms] = useState<Terms | null>(null);
  const [agreed, setAgreed] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    void api.terms().then(setTerms);
  }, []);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!terms) return;
    try {
      await api.agreeTerms(terms.version);
      await refreshMe();
    } catch (err) {
      setError((err as Error).message);
      setTerms(await api.terms());
      setAgreed(false);
    }
  };

  return (
    <section className="card narrow">
      <h1>利用規約が更新されました</h1>
      <p className="muted small">引き続き利用するには、新しい利用規約を読んで同意してください。</p>
      <form onSubmit={submit}>
        <div className="terms-box">{terms ? <Markdown text={terms.text} /> : <p className="muted">読み込み中…</p>}</div>
        <PolicyLink />
        <label className="checkbox">
          <input type="checkbox" checked={agreed} onChange={(e) => setAgreed(e.target.checked)} />
          利用規約と運営ポリシーに同意します
        </label>
        {error && <p className="error">{error}</p>}
        <button type="submit" className="button primary block" disabled={!agreed || !terms}>
          同意して続ける
        </button>
      </form>
    </section>
  );
}

/** プロジェクト共通の運営ポリシーへのリンク。 */
export function PolicyLink() {
  const { instance } = useApp();
  const base = instance?.sourceCodeUrl ?? "https://github.com/tacky0612/smash-hub";
  return (
    <p className="small">
      <a href={`${base}/blob/main/docs/policy.md`} rel="noreferrer">
        運営ポリシー（SmashHub プロジェクト共通）
      </a>
    </p>
  );
}
