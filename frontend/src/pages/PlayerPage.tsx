import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { api, type Match, type Standing } from "../api";
import { useApp } from "../context";

const STATUS_LABELS: Record<Match["status"], string> = {
  in_progress: "対戦中",
  confirmed: "",
  cancelled: "中止",
  expired: "期限切れ",
};

export function PlayerPage() {
  const { id = "" } = useParams();
  const { fighterName } = useApp();
  const [standing, setStanding] = useState<Standing | null>(null);
  const [history, setHistory] = useState<Match[]>([]);

  useEffect(() => {
    void api.standing(id).then(setStanding);
    void api.history(id).then(setHistory);
  }, [id]);

  if (!standing) return <p className="loading">読み込み中…</p>;
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
                <li key={m.id}>
                  <span className={`result ${result === "勝ち" ? "win" : result === "負け" ? "lose" : ""}`}>{result}</span>
                  <span>
                    {fighterName(self.fighter)} vs <Link to={`/players/${opp.player.id}`}>{opp.player.displayName}</Link>（
                    {fighterName(opp.fighter)}）
                  </span>
                  <time className="muted small">{new Date(m.playedAt).toLocaleString("ja-JP")}</time>
                </li>
              );
            })}
          </ul>
        )}
      </section>
    </>
  );
}
