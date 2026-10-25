import { useState, type FormEvent } from "react";
import { useSearchParams } from "react-router";
import { api } from "../api";
import { useApp } from "../context";

export function LoginPage() {
  const { instance, refreshMe } = useApp();
  const [params] = useSearchParams();
  const [name, setName] = useState("");
  const [error, setError] = useState(params.get("error") ? "ログインに失敗しました。もう一度お試しください。" : "");

  const devLogin = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      await api.devLogin(name);
      await refreshMe();
    } catch (err) {
      setError((err as Error).message);
    }
  };

  const providers = instance?.loginProviders;
  return (
    <section className="card narrow">
      <h1>ログイン</h1>
      <p className="muted">
        Smash-hub はメールアドレスやパスワードを保存しません。外部サービスのアカウントでログインします。
      </p>
      {error && <p className="error">{error}</p>}
      {providers?.discord && (
        <a className="button primary block" href="/api/auth/discord/start">
          Discord でログイン
        </a>
      )}
      {providers?.dev && (
        <form onSubmit={devLogin} className="dev-login">
          <h2>開発用ログイン</h2>
          <p className="muted">開発環境でだけ使えるログインです。名前ごとに別の利用者として扱います。</p>
          <label>
            名前
            <input value={name} onChange={(e) => setName(e.target.value)} maxLength={40} required />
          </label>
          <button type="submit" className="button block">
            ログイン
          </button>
        </form>
      )}
      {providers && !providers.dev && !providers.discord && (
        <p className="error">ログイン方法が設定されていません。インスタンスの運営者にお問い合わせください。</p>
      )}
    </section>
  );
}
