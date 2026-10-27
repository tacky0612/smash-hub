import { useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router";
import {
  api,
  CATEGORY_LABELS,
  SANCTION_CATEGORIES,
  SANCTION_LABELS,
  VOID_REASON_LABELS,
  type AccountRef,
  type ModerationReport,
  type Sanction,
  type SanctionKind,
} from "../api";

/** 管理画面: 通報の確認・処分・対戦記録の無効化（docs/domain/match-rules.md の「10. 通報とモデレーション」）。 */
export function AdminPage() {
  const [status, setStatus] = useState<"open" | "resolved">("open");
  const [reports, setReports] = useState<ModerationReport[]>([]);
  const [target, setTarget] = useState("");
  const [error, setError] = useState("");

  const load = async () => {
    try {
      setReports(await api.admin.reports(status));
      setError("");
    } catch (err) {
      setError((err as Error).message);
    }
  };
  useEffect(() => {
    void load();
  }, [status]);

  return (
    <>
      <section className="card">
        <h1>管理</h1>
        <div className="inline">
          <button className={`button ${status === "open" ? "primary" : ""}`} onClick={() => setStatus("open")}>
            未対応の通報
          </button>
          <button className={`button ${status === "resolved" ? "primary" : ""}`} onClick={() => setStatus("resolved")}>
            対応済み
          </button>
        </div>
        {error && <p className="error">{error}</p>}
        {reports.length === 0 && <p className="muted">通報はありません。</p>}
      </section>
      {reports.map((r) => (
        <ReportCard key={r.id} report={r} onChanged={load} onOpenAccount={setTarget} />
      ))}
      <AccountSanctions accountId={target} onAccountIdChange={setTarget} />
    </>
  );
}

function ReportCard({
  report: r,
  onChanged,
  onOpenAccount,
}: {
  report: ModerationReport;
  onChanged: () => void;
  onOpenAccount: (id: string) => void;
}) {
  const [note, setNote] = useState("");
  const [voidReason, setVoidReason] = useState("");
  const [message, setMessage] = useState("");

  const resolve = async () => {
    try {
      await api.admin.resolve(r.id, note);
      onChanged();
    } catch (err) {
      setMessage((err as Error).message);
    }
  };
  const voidMatch = async () => {
    if (!r.matchId || !voidReason) return;
    try {
      await api.admin.voidMatch(r.matchId, voidReason);
      setMessage("対戦を無効化しました。連合先にも伝わり、レート計算から除外されます。");
    } catch (err) {
      setMessage((err as Error).message);
    }
  };

  return (
    <section className="card">
      <h2>
        {CATEGORY_LABELS[r.category] ?? r.category}
        <span className="muted small"> {new Date(r.createdAt).toLocaleString("ja-JP")}</span>
      </h2>
      <dl className="room">
        <dt>対象</dt>
        <dd>
          <Link to={`/players/${r.target.id}`}>{r.target.displayName}</Link>{" "}
          <button className="link-button" onClick={() => onOpenAccount(r.target.id)}>
            処分の記録・処分する
          </button>
        </dd>
        <dt>通報者</dt>
        <dd>
          <Link to={`/players/${r.reporter.id}`}>{r.reporter.displayName}</Link>
        </dd>
        {r.matchId && (
          <>
            <dt>対戦</dt>
            <dd>
              {r.hostedHere ? <Link to={`/matches/${r.matchId}`}>対戦ルームを開く</Link> : <span>他のインスタンス（{r.matchHost}）がホスト</span>}
            </dd>
          </>
        )}
      </dl>
      <p className="report-body">{r.body}</p>
      {r.status === "resolved" ? (
        <p className="muted small">対応済み: {r.resolution || "（メモなし）"}</p>
      ) : (
        <>
          {r.matchId &&
            (r.hostedHere ? (
              <div className="inline">
                <select value={voidReason} onChange={(e) => setVoidReason(e.target.value)}>
                  <option value="">無効化の事由</option>
                  {Object.entries(VOID_REASON_LABELS).map(([k, v]) => (
                    <option key={k} value={k}>
                      {v}
                    </option>
                  ))}
                </select>
                <button className="button" onClick={voidMatch} disabled={!voidReason}>
                  対戦を無効化
                </button>
              </div>
            ) : (
              <p className="muted small">この対戦はホストのインスタンスの運営者しか無効化できません。必要ならホストの運営者に依頼してください。</p>
            ))}
          <p className="muted small">無効化できるのは対戦から7日以内です。無効化は連合で共有され、取り消せません。</p>
          <label>
            対応メモ（運営者だけが見る。500文字まで）
            <input value={note} onChange={(e) => setNote(e.target.value)} maxLength={500} />
          </label>
          <button className="button primary" onClick={resolve}>
            対応済みにする
          </button>
        </>
      )}
      {message && <p className="notice">{message}</p>}
    </section>
  );
}

function AccountSanctions({ accountId, onAccountIdChange }: { accountId: string; onAccountIdChange: (id: string) => void }) {
  const [account, setAccount] = useState<AccountRef | null>(null);
  const [sanctions, setSanctions] = useState<Sanction[]>([]);
  const [kind, setKind] = useState<SanctionKind>("caution");
  const [reason, setReason] = useState("");
  const [days, setDays] = useState(7);
  const [note, setNote] = useState("");
  const [error, setError] = useState("");

  const load = async () => {
    if (!accountId) return;
    try {
      const res = await api.admin.sanctions(accountId);
      setAccount(res.account);
      setSanctions(res.sanctions);
      setError("");
    } catch (err) {
      setAccount(null);
      setError((err as Error).message);
    }
  };
  useEffect(() => {
    void load();
  }, [accountId]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    try {
      await api.admin.sanction({ accountId, kind, reason, note, days: kind === "red" ? days : 0 });
      setNote("");
      await load();
    } catch (err) {
      setError((err as Error).message);
    }
  };
  const lift = async (id: string) => {
    try {
      await api.admin.lift(id);
      await load();
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <section className="card">
      <h2>処分</h2>
      <label>
        アカウント ID
        <input value={accountId} onChange={(e) => onAccountIdChange(e.target.value.trim())} placeholder="did:key:..." className="mono" />
      </label>
      {error && <p className="error">{error}</p>}
      {account && (
        <>
          <p>
            <Link to={`/players/${account.id}`}>{account.displayName}</Link> の処分の記録
          </p>
          {sanctions.length === 0 ? (
            <p className="muted">処分の記録はありません。</p>
          ) : (
            <ul className="history">
              {sanctions.map((s) => (
                <li key={s.id}>
                  <span className={s.active ? "badge" : ""}>{SANCTION_LABELS[s.kind]}</span>
                  <span>
                    {CATEGORY_LABELS[s.reason] ?? s.reason}
                    {s.note && <span className="muted small">（{s.note}）</span>}
                    {s.kind === "red" && (
                      <span className="muted small">
                        {" "}
                        {s.liftedAt ? "解除済み" : s.endsAt ? `${new Date(s.endsAt).toLocaleString("ja-JP")} まで` : "無期限"}
                      </span>
                    )}
                  </span>
                  <time className="muted small">{new Date(s.createdAt).toLocaleString("ja-JP")}</time>
                  {s.active && (
                    <button className="link-button" onClick={() => lift(s.id)}>
                      解除
                    </button>
                  )}
                </li>
              ))}
            </ul>
          )}
          <form onSubmit={submit} className="sanction-form">
            <label>
              段階
              <select value={kind} onChange={(e) => setKind(e.target.value as SanctionKind)}>
                {(Object.keys(SANCTION_LABELS) as SanctionKind[]).map((k) => (
                  <option key={k} value={k}>
                    {SANCTION_LABELS[k]}
                  </option>
                ))}
              </select>
            </label>
            {kind === "red" && (
              <label>
                期間（日数。0 は無期限）
                <input type="number" min={0} max={3650} value={days} onChange={(e) => setDays(Number(e.target.value))} />
              </label>
            )}
            <label>
              理由
              <select value={reason} onChange={(e) => setReason(e.target.value)} required>
                <option value="">選んでください</option>
                {SANCTION_CATEGORIES.map((c) => (
                  <option key={c} value={c}>
                    {CATEGORY_LABELS[c]}
                  </option>
                ))}
              </select>
            </label>
            <label>
              メモ（運営者だけが見る。500文字まで）
              <input value={note} onChange={(e) => setNote(e.target.value)} maxLength={500} />
            </label>
            <p className="muted small">段階と理由は本人にも表示します。メモは表示しません。</p>
            <button type="submit" className="button primary">
              処分する
            </button>
          </form>
        </>
      )}
    </section>
  );
}
