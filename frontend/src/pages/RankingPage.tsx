import { useEffect, useState } from "react";
import { Link } from "react-router";
import { api, type Ranking } from "../api";

export function RankingPage() {
  const [ranking, setRanking] = useState<Ranking | null>(null);
  useEffect(() => {
    void api.ranking().then(setRanking);
  }, []);
  if (!ranking) return <p className="loading">読み込み中…</p>;
  return (
    <section className="card">
      <h1>ランキング</h1>
      <p className="muted small">
        シーズン {ranking.season} ／ {ranking.minMatches}戦以上のプレイヤーを掲載しています。
      </p>
      {ranking.entries.length === 0 ? (
        <p>まだ掲載条件を満たすプレイヤーがいません。</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>順位</th>
                <th>プレイヤー</th>
                <th>レート</th>
                <th>戦績</th>
              </tr>
            </thead>
            <tbody>
              {ranking.entries.map((e) => (
                <tr key={e.accountId}>
                  <td>{e.rank}</td>
                  <td>
                    <Link to={`/players/${e.accountId}`}>{e.displayName}</Link>
                  </td>
                  <td>
                    <strong>{e.rating}</strong>
                  </td>
                  <td>
                    {e.wins}勝 {e.losses}敗
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
