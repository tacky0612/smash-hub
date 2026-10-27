// 図を描くための SVG の部品。図はすべてこの部品の組み合わせで描き、配色はトーン（Tone）で揃える。
// 画像ファイルや外部のライブラリを使わず、テキストと図形だけで表す（公式の素材を使わない方針）。
import { createContext, useContext, useId, type ReactNode } from "react";

export type Tone = "primary" | "neutral" | "accent" | "success" | "danger" | "muted";

const NODE_TONES: Record<Tone, { box: string; text: string }> = {
  primary: { box: "fill-indigo-50 stroke-indigo-400 dark:fill-indigo-950 dark:stroke-indigo-500", text: "fill-indigo-950 dark:fill-indigo-100" },
  neutral: { box: "fill-white stroke-slate-300 dark:fill-slate-900 dark:stroke-slate-600", text: "fill-slate-800 dark:fill-slate-100" },
  accent: { box: "fill-amber-50 stroke-amber-400 dark:fill-amber-950 dark:stroke-amber-500", text: "fill-amber-950 dark:fill-amber-100" },
  success: { box: "fill-emerald-50 stroke-emerald-500 dark:fill-emerald-950 dark:stroke-emerald-500", text: "fill-emerald-950 dark:fill-emerald-100" },
  danger: { box: "fill-rose-50 stroke-rose-400 dark:fill-rose-950 dark:stroke-rose-500", text: "fill-rose-950 dark:fill-rose-100" },
  muted: { box: "fill-slate-100 stroke-slate-300 dark:fill-slate-800 dark:stroke-slate-600", text: "fill-slate-500 dark:fill-slate-400" },
};

const LINE_TONES: Record<Tone, { stroke: string; fill: string }> = {
  primary: { stroke: "stroke-indigo-500 dark:stroke-indigo-400", fill: "fill-indigo-500 dark:fill-indigo-400" },
  neutral: { stroke: "stroke-slate-500 dark:stroke-slate-400", fill: "fill-slate-500 dark:fill-slate-400" },
  accent: { stroke: "stroke-amber-500 dark:stroke-amber-400", fill: "fill-amber-500 dark:fill-amber-400" },
  success: { stroke: "stroke-emerald-600 dark:stroke-emerald-400", fill: "fill-emerald-600 dark:fill-emerald-400" },
  danger: { stroke: "stroke-rose-500 dark:stroke-rose-400", fill: "fill-rose-500 dark:fill-rose-400" },
  muted: { stroke: "stroke-slate-300 dark:stroke-slate-600", fill: "fill-slate-300 dark:fill-slate-600" },
};

const TONES = Object.keys(LINE_TONES) as Tone[];

// 図ごとに一意な矢印の marker の ID（同じページに複数の図があっても衝突しない）
const MarkerContext = createContext("arrow");
const markerId = (prefix: string, tone: Tone) => `${prefix}-${tone}`;

type DiagramProps = {
  title: string;
  /** 図の説明（スクリーンリーダー向け・図の下に表示） */
  caption?: ReactNode;
  width: number;
  height: number;
  children: ReactNode;
  /** 表示の最小幅（これより狭い画面では横にスクロールする）。スマートフォン用の図では 0 にする */
  minWidth?: number;
};

/** 図の外枠。 */
export function Diagram({ title, caption, width, height, children, minWidth = Math.min(width, 640) }: DiagramProps) {
  const prefix = useId().replace(/:/g, "");
  return (
    <figure className="my-8">
      <div className="overflow-x-auto rounded-xl border border-slate-200 bg-slate-50/60 p-4 dark:border-slate-800 dark:bg-slate-900/60">
        <svg
          viewBox={`0 0 ${width} ${height}`}
          role="img"
          aria-label={title}
          className="mx-auto block h-auto w-full font-sans"
          style={{ minWidth, maxWidth: width }}
        >
          <title>{title}</title>
          <defs>
            {TONES.map((tone) => (
              <marker
                key={tone}
                id={markerId(prefix, tone)}
                viewBox="0 0 10 10"
                refX="9"
                refY="5"
                markerWidth="7"
                markerHeight="7"
                orient="auto-start-reverse"
              >
                <path d="M0,0 L10,5 L0,10 z" className={LINE_TONES[tone].fill} />
              </marker>
            ))}
          </defs>
          <MarkerContext.Provider value={prefix}>{children}</MarkerContext.Provider>
        </svg>
      </div>
      {caption && <figcaption className="mt-2 text-center text-sm text-slate-500 dark:text-slate-400">{caption}</figcaption>}
    </figure>
  );
}

type NodeProps = {
  x: number;
  y: number;
  w: number;
  h: number;
  /** 1行目を見出し（太字）、2行目以降を補足として表示する */
  lines: string[];
  tone?: Tone;
  rounded?: number;
  dashed?: boolean;
};

/** 箱（インスタンス・データ・処理など）。 */
export function Node({ x, y, w, h, lines, tone = "neutral", rounded = 10, dashed }: NodeProps) {
  const t = NODE_TONES[tone];
  const lineHeight = 17;
  const top = y + h / 2 - ((lines.length - 1) * lineHeight) / 2;
  return (
    <g>
      <rect x={x} y={y} width={w} height={h} rx={rounded} className={t.box} strokeWidth={1.5} strokeDasharray={dashed ? "5 4" : undefined} />
      {lines.map((line, i) => (
        <text
          key={i}
          x={x + w / 2}
          y={top + i * lineHeight}
          textAnchor="middle"
          dominantBaseline="middle"
          className={`${t.text} ${i === 0 ? "text-[14px] font-semibold" : "text-[12px] opacity-80"}`}
        >
          {line}
        </text>
      ))}
    </g>
  );
}

type ArrowProps = {
  from: [number, number];
  to: [number, number];
  label?: string;
  tone?: Tone;
  dashed?: boolean;
  /** 両方向の矢印 */
  both?: boolean;
  /** ラベルの位置の調整（線に対して上下・左右にずらす） */
  labelOffset?: [number, number];
};

/** 矢印。ラベルは線の中央に置く。 */
export function Arrow({ from, to, label, tone = "neutral", dashed, both, labelOffset = [0, -8] }: ArrowProps) {
  const prefix = useContext(MarkerContext);
  const marker = `url(#${markerId(prefix, tone)})`;
  const mx = (from[0] + to[0]) / 2 + labelOffset[0];
  const my = (from[1] + to[1]) / 2 + labelOffset[1];
  return (
    <g>
      <line
        x1={from[0]}
        y1={from[1]}
        x2={to[0]}
        y2={to[1]}
        className={LINE_TONES[tone].stroke}
        strokeWidth={1.8}
        strokeDasharray={dashed ? "6 5" : undefined}
        markerEnd={marker}
        markerStart={both ? marker : undefined}
      />
      {label && <Label x={mx} y={my} text={label} />}
    </g>
  );
}

/** 曲線などの自由な形の矢印（d は SVG の path）。 */
export function Connector({ d, tone = "neutral", dashed, both }: { d: string; tone?: Tone; dashed?: boolean; both?: boolean }) {
  const prefix = useContext(MarkerContext);
  const marker = `url(#${markerId(prefix, tone)})`;
  return (
    <path
      d={d}
      className={`fill-none ${LINE_TONES[tone].stroke}`}
      strokeWidth={1.8}
      strokeDasharray={dashed ? "6 5" : undefined}
      markerEnd={marker}
      markerStart={both ? marker : undefined}
    />
  );
}

/**
 * PC とスマートフォンで構成を変える図。md（768px）以上で desktop、それ未満で mobile を表示する。
 * スマートフォン用の図は縦に並べ、文字が小さくなりすぎないよう幅 340 前後で描く。
 */
export function Responsive({ desktop, mobile }: { desktop: ReactNode; mobile: ReactNode }) {
  return (
    <>
      <div className="hidden md:block">{desktop}</div>
      <div className="md:hidden">{mobile}</div>
    </>
  );
}

/** 背景付きのラベル（線の上でも読めるようにする）。 */
export function Label({ x, y, text, anchor = "middle" }: { x: number; y: number; text: string; anchor?: "start" | "middle" | "end" }) {
  // 全角文字を考慮したおおよその幅
  const width = [...text].reduce((n, c) => n + (c.charCodeAt(0) > 0xff ? 12 : 7), 0) + 10;
  const left = anchor === "middle" ? x - width / 2 : anchor === "end" ? x - width : x;
  return (
    <g>
      <rect x={left} y={y - 10} width={width} height={20} rx={4} className="fill-white/90 dark:fill-slate-950/90" />
      <text x={left + width / 2} y={y} textAnchor="middle" dominantBaseline="middle" className="fill-slate-600 text-[12px] dark:fill-slate-300">
        {text}
      </text>
    </g>
  );
}

/** 図の中の見出し・注記。 */
export function Caption({ x, y, text, anchor = "middle", strong }: { x: number; y: number; text: string; anchor?: "start" | "middle" | "end"; strong?: boolean }) {
  return (
    <text
      x={x}
      y={y}
      textAnchor={anchor}
      dominantBaseline="middle"
      className={strong ? "fill-slate-800 text-[14px] font-bold dark:fill-slate-100" : "fill-slate-500 text-[12px] dark:fill-slate-400"}
    >
      {text}
    </text>
  );
}

/** 領域（サイトの内側・連合の範囲など）を示す破線の枠。 */
export function Region({ x, y, w, h, label, tone = "muted" }: { x: number; y: number; w: number; h: number; label?: string; tone?: Tone }) {
  return (
    <g>
      <rect x={x} y={y} width={w} height={h} rx={14} className={`fill-transparent ${LINE_TONES[tone].stroke}`} strokeWidth={1.5} strokeDasharray="6 5" />
      {label && (
        <text x={x + 14} y={y + 18} dominantBaseline="middle" className="fill-slate-500 text-[12px] font-semibold dark:fill-slate-400">
          {label}
        </text>
      )}
    </g>
  );
}
