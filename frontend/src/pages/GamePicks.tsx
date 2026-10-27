import type { Match, Pick } from "../api";
import { useApp } from "../context";
import { FighterSelect } from "./FighterSelect";

/** 報告するゲームごとのファイターの初期値。前回の報告があればそれを、なければ1戦目に登録したファイターを入れる。 */
export function initialPicks(match: Match, mine?: Pick[]): Pick[] {
  if (mine && mine.length > 0) return mine;
  const first: [string, string] = [real(match.player1.fighter), real(match.player2.fighter)];
  return [
    { game: 1, characters: first },
    { game: 2, characters: [...first] },
  ];
}

// おまかせで登録した側は、実際に使ったファイターを選んでもらう
function real(fighter: string) {
  return fighter === "random" ? "" : fighter;
}

type Props = {
  match: Match;
  picks: Pick[];
  onChange: (picks: Pick[]) => void;
};

/**
 * 結果報告で入力するゲームごとのファイター（任意）。両者の入力が一致したときだけ記録に含める。
 * 1戦目はマッチングで選んだファイター（おまかせの側だけ選び直す）。
 */
export function GamePicksInput({ match, picks, onChange }: Props) {
  const setCount = (n: number) => {
    const next = picks.slice(0, n);
    while (next.length < n) {
      const prev = next[next.length - 1];
      next.push({ game: next.length + 1, characters: [...prev.characters] });
    }
    onChange(next);
  };
  const setFighter = (game: number, side: 0 | 1, fighter: string) =>
    onChange(
      picks.map((p, i) => {
        if (i !== game) return p;
        const characters: [string, string] = [...p.characters];
        characters[side] = fighter;
        return { ...p, characters };
      }),
    );
  const locked = [match.player1.fighter !== "random", match.player2.fighter !== "random"];

  return (
    <div className="game-picks">
      <div className="inline">
        <span>ゲーム数</span>
        {[2, 3].map((n) => (
          <label key={n} className="radio">
            <input type="radio" name="games" checked={picks.length === n} onChange={() => setCount(n)} />
            {n}ゲーム
          </label>
        ))}
      </div>
      {picks.map((p, i) => (
        <fieldset key={p.game} className="game-row">
          <legend>{p.game}戦目</legend>
          {i === 0 && locked[0] ? (
            <FixedFighter label={match.player1.player.displayName} fighter={p.characters[0]} />
          ) : (
            <FighterSelect label={match.player1.player.displayName} value={p.characters[0]} onChange={(f) => setFighter(i, 0, f)} />
          )}
          {i === 0 && locked[1] ? (
            <FixedFighter label={match.player2.player.displayName} fighter={p.characters[1]} />
          ) : (
            <FighterSelect label={match.player2.player.displayName} value={p.characters[1]} onChange={(f) => setFighter(i, 1, f)} />
          )}
        </fieldset>
      ))}
      <p className="muted small">2・3戦目にファイターを変えられるのは直前の敗者だけです。</p>
    </div>
  );
}

function FixedFighter({ label, fighter }: { label: string; fighter: string }) {
  const { fighterName } = useApp();
  return (
    <label>
      {label}
      <span className="fixed">{fighterName(fighter)}</span>
    </label>
  );
}

/** 記録したゲームごとのファイターの表示。 */
export function RecordedPicks({ match }: { match: Match }) {
  const { fighterName } = useApp();
  if (!match.recordedPicks || match.recordedPicks.length === 0) return null;
  return (
    <section className="card">
      <h2>記録したファイター</h2>
      <table className="picks">
        <thead>
          <tr>
            <th></th>
            <th>{match.player1.player.displayName}</th>
            <th>{match.player2.player.displayName}</th>
          </tr>
        </thead>
        <tbody>
          {match.recordedPicks.map((p) => (
            <tr key={p.game}>
              <th>{p.game}戦目</th>
              <td>{fighterName(p.characters[0])}</td>
              <td>{fighterName(p.characters[1])}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {match.recordedPicks.length === 1 && (
        <p className="muted small">ゲームごとのファイターは両者の入力がそろわなかったため、1戦目だけを記録しました。</p>
      )}
    </section>
  );
}
