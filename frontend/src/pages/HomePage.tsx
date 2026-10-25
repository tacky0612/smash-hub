import { useEffect, useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { api, type MatchmakingStatus, type Standing } from "../api";
import { useApp, usePolling } from "../context";
import { FighterSelect } from "./FighterSelect";

const RATE_RANGES = [
  { value: 0, label: "制限なし（±200から1分ごとに広がる）" },
  { value: 50, label: "±50" },
  { value: 100, label: "±100" },
  { value: 150, label: "±150" },
  { value: 200, label: "±200" },
];

export function HomePage() {
  const { me } = useApp();
  const navigate = useNavigate();
  const account = me?.loggedIn && me.registered ? me.account : null;
  const [standing, setStanding] = useState<Standing | null>(null);
  const [fighter, setFighter] = useState(account?.mainFighter ?? "");
  const [rateRange, setRateRange] = useState(0);
  const [status, setStatus] = useState<MatchmakingStatus>({ state: "idle" });
  const [error, setError] = useState("");

  useEffect(() => {
    if (account) void api.standing(account.id).then(setStanding);
  }, [account]);

  const apply = (st: MatchmakingStatus) => {
    setStatus(st);
    if (st.state === "matched" && st.matchId) {
      // マッチング成立を音で知らせる（docs/architecture/frontend.md）
      beep();
      navigate(`/matches/${st.matchId}`);
    }
  };

  // マッチング待ちの間は状態を確認し続ける（確認がないと離脱したとみなされる）
  usePolling(
    async () => {
      const st = await api.status();
      if (st.state === "matched" && st.matchId) {
        if (status.state === "waiting") beep();
        navigate(`/matches/${st.matchId}`);
        return false;
      }
      setStatus(st);
      return st.state === "waiting";
    },
    3000,
    [status.state === "waiting"],
  );

  const search = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      apply(await api.search(fighter, rateRange));
    } catch (err) {
      setError((err as Error).message);
    }
  };

  const cancel = async () => {
    await api.cancelSearch();
    setStatus({ state: "idle" });
  };

  return (
    <>
      <section className="card">
        <h1>{account?.displayName}</h1>
        {standing && (
          <p className="rating">
            レート <strong>{standing.rating}</strong>
            <span className="muted">
              {standing.wins}勝 {standing.losses}敗
            </span>
          </p>
        )}
      </section>

      <section className="card">
        <h2>レーティング対戦</h2>
        {status.state === "waiting" ? (
          <div className="waiting">
            <p className="pulse">対戦相手を探しています…</p>
            <p className="muted">
              待ち時間 {status.waitingSeconds ?? 0}秒 ／ レート差 ±{status.allowedDiff}まで
            </p>
            <p className="muted small">この画面を開いたままにしてください。閉じるとマッチングが取り消されます。</p>
            <button className="button block" onClick={cancel}>
              やめる
            </button>
          </div>
        ) : (
          <form onSubmit={search}>
            <FighterSelect label="1戦目のファイター" value={fighter} onChange={setFighter} allowRandom />
            <label>
              マッチングするレート差
              <select value={rateRange} onChange={(e) => setRateRange(Number(e.target.value))}>
                {RATE_RANGES.map((r) => (
                  <option key={r.value} value={r.value}>
                    {r.label}
                  </option>
                ))}
              </select>
            </label>
            {error && <p className="error">{error}</p>}
            <button type="submit" className="button primary block">
              対戦相手を探す
            </button>
          </form>
        )}
      </section>
    </>
  );
}

function beep() {
  try {
    const ctx = new AudioContext();
    const osc = ctx.createOscillator();
    osc.frequency.value = 880;
    osc.connect(ctx.destination);
    osc.start();
    osc.stop(ctx.currentTime + 0.2);
  } catch {
    // 音が出せない環境では何もしない
  }
}
