// データから描くシーケンス図。参加者とメッセージの並びを渡すと、ライフラインと矢印を配置する。
// プロトコルの流れが変わったら、図ではなくデータ（participants・messages）を直す。
// PC では SVG のシーケンス図、スマートフォンでは同じデータから縦の時系列（HTML）を表示する。
import { ArrowRight, RotateCcw } from "lucide-react";
import { Arrow, Diagram, Label, Node, Responsive, type Tone } from "./kit";

export type Participant = {
  id: string;
  /** 1行目が名前、2行目以降が補足 */
  lines: string[];
  tone?: Tone;
};

export type Message =
  | { from: string; to: string; label: string; tone?: Tone; dashed?: boolean }
  /** 区切り（フェーズの見出し） */
  | { section: string };

type Props = {
  title: string;
  caption?: string;
  participants: Participant[];
  messages: Message[];
};

const COL = 190;
const HEAD = 64;
const ROW = 46;
const PAD = 20;

export function SequenceDiagram(props: Props) {
  return <Responsive desktop={<SequenceSvg {...props} />} mobile={<SequenceTimeline {...props} />} />;
}

function SequenceSvg({ title, caption, participants, messages }: Props) {
  const width = PAD * 2 + COL * participants.length;
  const height = PAD + HEAD + 24 + messages.length * ROW + PAD;
  const cx = (id: string) => {
    const i = participants.findIndex((p) => p.id === id);
    if (i < 0) throw new Error(`unknown participant: ${id}`);
    return PAD + COL * i + COL / 2;
  };
  const lifelineTop = PAD + HEAD;
  const lifelineBottom = height - PAD;

  return (
    <Diagram title={title} caption={caption} width={width} height={height}>
      {participants.map((p) => (
        <g key={p.id}>
          <line
            x1={cx(p.id)}
            y1={lifelineTop}
            x2={cx(p.id)}
            y2={lifelineBottom}
            className="stroke-slate-300"
            strokeWidth={1.5}
            strokeDasharray="4 4"
          />
          <Node x={cx(p.id) - COL / 2 + 10} y={PAD} w={COL - 20} h={HEAD - 8} lines={p.lines} tone={p.tone ?? "neutral"} />
        </g>
      ))}
      {messages.map((m, i) => {
        const y = lifelineTop + 24 + i * ROW + ROW / 2;
        if ("section" in m) {
          return (
            <g key={i}>
              <line x1={PAD} y1={y} x2={width - PAD} y2={y} className="stroke-slate-200" strokeWidth={8} />
              <Label x={PAD + 8} y={y} text={m.section} anchor="start" />
            </g>
          );
        }
        if (m.from === m.to) {
          // 自分自身への処理
          const x = cx(m.from);
          return (
            <g key={i}>
              <path
                d={`M${x},${y - 12} h36 v24 h-34`}
                className="fill-none stroke-slate-500"
                strokeWidth={1.8}
              />
              <Label x={x + 44} y={y} text={m.label} anchor="start" />
            </g>
          );
        }
        const dir = cx(m.to) > cx(m.from) ? 1 : -1;
        return (
          <Arrow
            key={i}
            from={[cx(m.from) + dir * 4, y]}
            to={[cx(m.to) - dir * 4, y]}
            label={m.label}
            tone={m.tone ?? "neutral"}
            dashed={m.dashed}
            labelOffset={[0, -11]}
          />
        );
      })}
    </Diagram>
  );
}

const CHIP_TONES: Record<Tone, string> = {
  primary: "bg-red-100 text-red-800",
  neutral: "bg-white text-slate-700 ring-1 ring-slate-300",
  accent: "bg-slate-800 text-white",
  success: "bg-white text-red-700 ring-1 ring-red-300",
  danger: "bg-slate-200 text-slate-600",
  muted: "bg-slate-100 text-slate-600",
};

/** スマートフォン用: 誰から誰へ何を送るかを、上から順に並べる。 */
function SequenceTimeline({ title, caption, participants, messages }: Props) {
  const byId = new Map(participants.map((p) => [p.id, p]));
  const chip = (id: string) => {
    const p = byId.get(id);
    if (!p) throw new Error(`unknown participant: ${id}`);
    return <span className={`inline-flex items-center rounded-md px-2 py-0.5 text-xs font-semibold ${CHIP_TONES[p.tone ?? "neutral"]}`}>{p.lines[0]}</span>;
  };
  let step = 0;
  return (
    <figure className="my-8" aria-label={title}>
      <div className="rounded-xl border border-slate-200 bg-slate-50/60 p-4">
        <p className="text-sm font-semibold text-slate-900">{title}</p>
        <ul className="mt-3 flex flex-wrap gap-2 text-xs text-slate-500">
          {participants.map((p) => (
            <li key={p.id} className="flex items-center gap-1">
              {chip(p.id)}
              <span>{p.lines.slice(1).join(" ")}</span>
            </li>
          ))}
        </ul>
        <ol className="mt-4 border-l-2 border-slate-200">
          {messages.map((m, i) => {
            if ("section" in m) {
              return (
                <li key={i} className="-ml-px mt-4 border-l-2 border-red-400 py-1 pl-3 text-xs font-bold tracking-wide text-red-700 first:mt-0">
                  {m.section}
                </li>
              );
            }
            step += 1;
            return (
              <li key={i} className="relative py-2 pl-5">
                <span className="absolute -left-[9px] top-3 flex h-4 w-4 items-center justify-center rounded-full bg-white text-[10px] font-bold text-slate-500 ring-2 ring-slate-300">
                  {step}
                </span>
                <div className="flex flex-wrap items-center gap-1.5">
                  {chip(m.from)}
                  {m.from === m.to ? (
                    <RotateCcw className="h-3.5 w-3.5 text-slate-400" aria-label="自分で処理" />
                  ) : (
                    <>
                      <ArrowRight className="h-3.5 w-3.5 text-slate-400" aria-label="から" />
                      {chip(m.to)}
                    </>
                  )}
                </div>
                <p className={`mt-1 text-sm ${m.dashed ? "text-slate-500" : "text-slate-800"}`}>{m.label}</p>
              </li>
            );
          })}
        </ol>
      </div>
      {caption && <figcaption className="mt-2 text-center text-sm text-slate-500">{caption}</figcaption>}
    </figure>
  );
}
