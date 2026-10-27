// 直線的な手順の図（HTML）。
// 3つまでの手順は、PC で横に並べる。4つ以上は横に並べると窮屈なので、どの画面でも縦に並べ、
// PC では見出しと説明を左右に置く。スマートフォンでは常に縦に並べる。
import { ChevronDown, ChevronRight } from "lucide-react";
import type { ReactNode } from "react";

export type Step = {
  title: string;
  body: ReactNode;
  /** 手順の補足（API 名など） */
  tag?: string;
};

type Props = {
  title: string;
  steps: Step[];
};

export function StepFlow({ title, steps }: Props) {
  const horizontal = steps.length <= 3;
  return (
    <figure className="my-8" aria-label={title}>
      <ol className={`grid gap-4 ${horizontal ? "md:grid-cols-3 md:gap-5" : ""}`}>
        {steps.map((s, i) => (
          <li
            key={s.title}
            className={`relative rounded-xl border border-slate-200 bg-white p-4 shadow-sm ${
              horizontal ? "flex flex-col" : "md:flex md:items-start md:gap-6"
            }`}
          >
            <div className={`mb-2 flex items-center gap-2 ${horizontal ? "" : "md:mb-0 md:w-56 md:shrink-0"}`}>
              <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-red-600 text-sm font-bold text-white">{i + 1}</span>
              <span className="font-semibold text-slate-900">{s.title}</span>
            </div>
            <div className="flex-1">
              <div className="text-sm leading-relaxed text-slate-600">{s.body}</div>
              {s.tag && <code className="mt-3 inline-block rounded bg-slate-100 px-2 py-0.5 text-xs break-all text-slate-700">{s.tag}</code>}
            </div>
            {i < steps.length - 1 && <Connector horizontal={horizontal} />}
          </li>
        ))}
      </ol>
      <figcaption className="sr-only">{title}</figcaption>
    </figure>
  );
}

/** 手順のあいだの矢印。横並びのときだけ PC で右向きにする。 */
function Connector({ horizontal }: { horizontal: boolean }) {
  const base =
    "absolute z-10 flex h-6 w-6 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-400";
  const position = horizontal
    ? "-bottom-[1.1rem] left-1/2 -translate-x-1/2 md:top-1/2 md:-right-[1.1rem] md:bottom-auto md:left-auto md:translate-x-0 md:-translate-y-1/2"
    : "-bottom-[1.1rem] left-1/2 -translate-x-1/2";
  return (
    <span aria-hidden className={`${base} ${position}`}>
      <ChevronDown className={`h-4 w-4 ${horizontal ? "md:hidden" : ""}`} />
      {horizontal && <ChevronRight className="hidden h-4 w-4 md:block" />}
    </span>
  );
}
