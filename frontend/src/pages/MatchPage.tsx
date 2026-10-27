import { useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router";
import { api, type Match, type MatchSide, type Pick, type Report } from "../api";
import { useApp, usePolling } from "../context";
import { GamePicksInput, initialPicks, RecordedPicks } from "./GamePicks";

const REPORT_LABELS: Record<Report, string> = { win: "勝ち", lose: "負け", cancel: "対戦中止" };

export function MatchPage() {
  const { id = "" } = useParams();
  const { fighterName } = useApp();
  const [match, setMatch] = useState<Match | null>(null);
  const [roomId, setRoomId] = useState("");
  const [error, setError] = useState("");
  // ゲームごとのファイター（任意）。null は入力しない
  const [picks, setPicks] = useState<Pick[] | null>(null);
  const picksRestored = useRef(false);

  // 前に報告したゲームごとのファイターがあれば、入力欄に戻す（最初に読み込んだときだけ）
  useEffect(() => {
    if (!match || picksRestored.current) return;
    picksRestored.current = true;
    const mine = match.myRole === "player1" ? match.player1 : match.player2;
    if (mine.picks?.length) setPicks(mine.picks);
  }, [match]);

  // 相手の入室・報告を反映するため、対戦中は状態を確認し続ける
  usePolling(
    async () => {
      const m = await api.match(id);
      setMatch(m);
      return m.status === "in_progress";
    },
    3000,
    [id],
  );

  if (!match) return <p className="loading">読み込み中…</p>;

  const mine = match.myRole === "player1" ? match.player1 : match.player2;
  const opponent = match.myRole === "player1" ? match.player2 : match.player1;
  const active = match.status === "in_progress";

  const report = (r: Report) => {
    if (r !== "cancel" && picks && picks.some((p) => p.characters.some((c) => !c))) {
      setError("ゲームごとのファイターをすべて選んでください（記録しない場合はチェックを外してください）");
      return;
    }
    void run(() => api.report(id, r, r === "cancel" ? undefined : (picks ?? undefined)));
  };

  const run = async (fn: () => Promise<Match>) => {
    setError("");
    try {
      setMatch(await fn());
    } catch (err) {
      setError((err as Error).message);
    }
  };

  const submitRoom = (e: FormEvent) => {
    e.preventDefault();
    void run(() => api.setRoom(id, roomId));
  };

  return (
    <>
      <section className="card">
        <h1>対戦ルーム</h1>
        <div className="versus">
          <PlayerBox side={match.player1} label="プレイヤー1（部屋を立てる）" fighterName={fighterName} />
          <span className="vs">VS</span>
          <PlayerBox side={match.player2} label="プレイヤー2（部屋に入る）" fighterName={fighterName} />
        </div>
        <StatusMessage match={match} />
      </section>

      {active && match.myRole && (
        <>
          <section className="card">
            <h2>専用部屋</h2>
            <dl className="room">
              <dt>部屋 ID</dt>
              <dd>{match.roomId ? <strong className="mono">{match.roomId}</strong> : <span className="muted">未入力</span>}</dd>
              <dt>パスワード</dt>
              <dd className="mono">{match.roomPassword}</dd>
            </dl>
            {match.myRole === "player1" ? (
              <form onSubmit={submitRoom} className="inline">
                <input
                  value={roomId}
                  onChange={(e) => setRoomId(e.target.value.toUpperCase())}
                  placeholder="部屋 ID（英数字5文字）"
                  maxLength={5}
                  pattern="[A-Za-z0-9]{5}"
                  required
                />
                <button type="submit" className="button">
                  {match.roomId ? "更新" : "入力"}
                </button>
              </form>
            ) : (
              <p className="muted small">プレイヤー1が部屋 ID を入力したら、その部屋に入ってください。</p>
            )}
            <details className="rules">
              <summary>部屋の設定とルール</summary>
              <ul>
                <li>ストック制・3ストック・7分、2本先取</li>
                <li>アイテム・スピリッツ・チャージ切りふだ・ステージギミック: なし</li>
                <li>入室制限: だれでも ／ 部屋の人数: 2人</li>
                <li>1戦目は両者がステージを選び、ゲームの抽選に従う。2・3戦目は直前の敗者が選ぶ</li>
                <li>2・3戦目にファイターを変えられるのは直前の敗者だけ</li>
              </ul>
            </details>
          </section>

          <section className="card">
            <h2>結果の報告</h2>
            <p className="muted small">2本先取で決着したら報告してください。両者の報告が一致すると確定します。</p>
            {mine.report && <p>あなたの報告: <strong>{REPORT_LABELS[mine.report]}</strong>（報告し直せます）</p>}
            <p className="muted small">相手: {opponent.reported ? "報告済み" : "未報告"}</p>
            <label className="checkbox">
              <input
                type="checkbox"
                checked={picks !== null}
                onChange={(e) => setPicks(e.target.checked ? initialPicks(match, mine.picks) : null)}
              />
              ゲームごとのファイターも記録する（任意）
            </label>
            {picks && (
              <GamePicksInput
                match={match}
                picks={picks}
                onChange={(next) => {
                  setPicks(next);
                  setError("");
                }}
              />
            )}
            <p className="muted small">
              勝ち負けが一致すれば確定します。ゲームごとのファイターは、両者の入力が一致したときだけ記録します（相手も同じ内容を入力してください）。
            </p>
            <div className="report-buttons">
              {(["win", "lose", "cancel"] as Report[]).map((r) => (
                <button
                  key={r}
                  className={`button ${r === "win" ? "primary" : ""} ${mine.report === r ? "selected" : ""}`}
                  onClick={() => report(r)}
                >
                  {REPORT_LABELS[r]}
                </button>
              ))}
            </div>
          </section>
        </>
      )}
      {match.status === "confirmed" && <RecordedPicks match={match} />}
      {error && <p className="error">{error}</p>}
      {!active && (
        <Link to="/" className="button primary block">
          次の対戦へ
        </Link>
      )}
    </>
  );
}

function PlayerBox({ side, label, fighterName }: { side: MatchSide; label: string; fighterName: (id: string) => string }) {
  return (
    <div className="player-box">
      <span className="muted small">{label}</span>
      <Link to={`/players/${side.player.id}`} className="player-name">
        {side.player.displayName}
      </Link>
      <span className="muted">レート {side.player.rating}</span>
      <span>1戦目: {fighterName(side.fighter)}</span>
    </div>
  );
}

function StatusMessage({ match }: { match: Match }) {
  switch (match.status) {
    case "confirmed": {
      const winner = match.winner === match.player1.player.id ? match.player1 : match.player2;
      if (match.voided)
        return (
          <p className="notice warning">
            {winner.player.displayName} の勝ちで確定しましたが、運営者が無効化したため、レートには含めていません。
          </p>
        );
      return <p className="notice success">{winner.player.displayName} の勝ちで確定しました。レートに反映されています。</p>;
    }
    case "cancelled":
      return <p className="notice">対戦は中止になりました（レート変動なし）。</p>;
    case "expired":
      return <p className="notice">24時間以内に報告がそろわなかったため、レート変動なしで終了しました。</p>;
    default:
      if (match.mismatched)
        return (
          <p className="notice warning">
            報告が食い違っています（{label(match.player1.report)} ／ {label(match.player2.report)}）。メッセージ等で確認し、間違えた方が報告し直してください。
          </p>
        );
      return null;
  }
}

function label(r?: Report) {
  return r ? REPORT_LABELS[r] : "未報告";
}
