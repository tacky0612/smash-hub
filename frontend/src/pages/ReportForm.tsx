import { useState, type FormEvent } from "react";
import { api, CATEGORY_LABELS, REPORT_CATEGORIES, type Match } from "../api";

/** 相手のユーザーページから送る通報（docs/domain/match-rules.md の「10.1 通報」）。 */
export function ReportForm({ targetId, matches, onDone }: { targetId: string; matches: Match[]; onDone: () => void }) {
  const [category, setCategory] = useState("");
  const [matchKey, setMatchKey] = useState("");
  const [body, setBody] = useState("");
  const [error, setError] = useState("");
  const [sent, setSent] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    const m = matches.find((x) => `${x.host} ${x.id}` === matchKey);
    try {
      await api.sendReport({ targetId, category, body, matchHost: m?.host, matchId: m?.id });
      setSent(true);
    } catch (err) {
      setError((err as Error).message);
    }
  };

  if (sent)
    return (
      <div>
        <p className="notice success">通報を受け付けました。運営者が確認します。通報したことは相手に知らせません。</p>
        <button className="button block" onClick={onDone}>
          閉じる
        </button>
      </div>
    );
  return (
    <form onSubmit={submit}>
      <label>
        区分
        <select value={category} onChange={(e) => setCategory(e.target.value)} required>
          <option value="">選んでください</option>
          {REPORT_CATEGORIES.map((c) => (
            <option key={c} value={c}>
              {CATEGORY_LABELS[c]}
            </option>
          ))}
        </select>
      </label>
      <label>
        対象の対戦（任意）
        <select value={matchKey} onChange={(e) => setMatchKey(e.target.value)}>
          <option value="">指定しない</option>
          {matches.map((m) => (
            <option key={`${m.host} ${m.id}`} value={`${m.host} ${m.id}`}>
              {new Date(m.playedAt).toLocaleString("ja-JP")}
            </option>
          ))}
        </select>
      </label>
      <label>
        状況（1〜1000文字）
        <textarea value={body} onChange={(e) => setBody(e.target.value)} maxLength={1000} rows={5} required />
      </label>
      <p className="muted small">
        通報の内容は運営者だけが見ます。対応が終わってから30日で削除します。本名や連絡先など、個人を特定できる情報は書かないでください。
      </p>
      {error && <p className="error">{error}</p>}
      <div className="inline">
        <button type="submit" className="button primary">
          通報する
        </button>
        <button type="button" className="button" onClick={onDone}>
          やめる
        </button>
      </div>
    </form>
  );
}
