import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { api, type Match, type Standing } from "../api";
import { useApp } from "../context";
import { ReportForm } from "./ReportForm";

const STATUS_LABELS: Record<Match["status"], string> = {
  in_progress: "対戦中",
  confirmed: "",
  cancelled: "中止",
  expired: "期限切れ",
};

export function PlayerPage() {
  const { id = "" } = useParams();
  const { me, fighterName } = useApp();
  const myId = me?.loggedIn && me.registered ? me.account.id : "";
  const [standing, setStanding] = useState<Standing | null>(null);
  const [history, setHistory] = useState<Match[]>([]);
  const [reporting, setReporting] = useState(false);

  useEffect(() => {
    setReporting(false);
    void api.standing(id).then(setStanding);
    void api.history(id).then(setHistory);
  }, [id]);

  if (!standing) return <p className="loading">読み込み中…</p>;
  // 通報の対象に選べるのは、自分との対戦だけ
  const withMe = history.filter((m) => m.player1.player.id === myId || m.player2.player.id === myId);
  return (
    <>
      <section className="card">
        <h1>{standing.displayName}</h1>
        <p className="rating">
          レート <strong>{standing.rating}</strong>
          <span className="muted">
            {standing.matches}戦 {standing.wins}勝 {standing.losses}敗
          </span>
        </p>
      </section>
      <section className="card">
        <h2>最近の対戦</h2>
        {history.length === 0 ? (
          <p className="muted">まだ対戦がありません。</p>
        ) : (
          <ul className="history">
            {history.map((m) => {
              const meIs1 = m.player1.player.id === id;
              const self = meIs1 ? m.player1 : m.player2;
              const opp = meIs1 ? m.player2 : m.player1;
              const result =
                m.status === "confirmed" ? (m.winner === id ? "勝ち" : "負け") : STATUS_LABELS[m.status];
              return (
                <li key={`${m.host} ${m.id}`}>
                  <span className={`result ${m.voided ? "" : result === "勝ち" ? "win" : result === "負け" ? "lose" : ""}`}>
                    {result}
                  </span>
                  <span>
                    {fighterName(self.fighter)} vs <Link to={`/players/${opp.player.id}`}>{opp.player.displayName}</Link>（
                    {fighterName(opp.fighter)}）{m.voided && <span className="badge">無効</span>}
                  </span>
                  <time className="muted small">{new Date(m.playedAt).toLocaleString("ja-JP")}</time>
                </li>
              );
            })}
          </ul>
        )}
        {history.some((m) => m.voided) && (
          <p className="muted small">「無効」の対戦は、運営者が無効化したためレートに含めていません。</p>
        )}
      </section>
      {myId && myId !== id && (
        <section className="card">
          <h2>通報</h2>
          {reporting ? (
            <ReportForm targetId={id} matches={withMe} onDone={() => setReporting(false)} />
          ) : (
            <>
              <p className="muted small">禁止事項に当たる行為があった場合は、運営者に通報してください。通報したことは相手に知らせません。</p>
              <button className="button" onClick={() => setReporting(true)}>
                このプレイヤーを通報する
              </button>
            </>
          )}
        </section>
      )}
    </>
  );
}
