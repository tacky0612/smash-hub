import { useApp } from "../context";

type Props = {
  value: string;
  onChange: (id: string) => void;
  allowRandom?: boolean;
  label: string;
};

/** ファイターの選択。画像は使わずテキストで表示する（docs/policy.md）。 */
export function FighterSelect({ value, onChange, allowRandom = false, label }: Props) {
  const { fighters } = useApp();
  return (
    <label>
      {label}
      <select value={value} onChange={(e) => onChange(e.target.value)} required>
        <option value="" disabled>
          選んでください
        </option>
        {allowRandom && <option value="random">おまかせ</option>}
        {fighters.map((f) => (
          <option key={f.id} value={f.id}>
            {f.name}
          </option>
        ))}
      </select>
    </label>
  );
}
