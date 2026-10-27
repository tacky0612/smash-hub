// 既存サイトの段階的な導入（記録の公開 → 取り込みと連合レート → インスタンスをまたぐ対戦）。
import { Network, RefreshCw, Upload, type LucideIcon } from "lucide-react";

type Stage = {
  icon: LucideIcon;
  name: string;
  gain: string;
  build: string[];
};

const STAGES: Stage[] = [
  {
    icon: Upload,
    name: "記録の公開",
    gain: "自サイトの記録が連合に複製され、閉鎖しても残る。他のサイトのランキングに自サイトの利用者が載る。",
    build: ["利用者ごとの鍵ペア（サーバー側）", "AccountPublished・MatchRecorded のログ", "GET /federation/instance・/log"],
  },
  {
    icon: RefreshCw,
    name: "取り込みと連合レート",
    gain: "他のサイトの記録を取り込み、サイトをまたいで比べられる連合レートとランキングを表示する。",
    build: ["ログの pull と検証", "連合レートの計算（整数演算）"],
  },
  {
    icon: Network,
    name: "インスタンスをまたぐ対戦",
    gain: "自サイトに相手がいなくても、連合全体の利用者とマッチングする。",
    build: ["インスタンス間リクエストの署名", "募集の配信・申し込み・状態の取得・報告の中継"],
  },
];

export function AdoptionStages() {
  return (
    <figure className="my-8" aria-label="既存サイトの3段階の導入">
      <ol className="grid gap-4 md:grid-cols-3">
        {STAGES.map((s, i) => (
          <li
            key={s.name}
            className="flex flex-col rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"
          >
            <div className="flex items-center gap-3 border-b border-slate-200 p-4 dark:border-slate-800">
              <span className="flex h-10 w-10 items-center justify-center rounded-lg bg-indigo-600 text-white">
                <s.icon className="h-5 w-5" strokeWidth={1.8} aria-hidden />
              </span>
              <div>
                <p className="text-xs font-semibold text-indigo-600 dark:text-indigo-400">段階 {i + 1}</p>
                <p className="font-semibold text-slate-900 dark:text-white">{s.name}</p>
              </div>
            </div>
            <div className="flex flex-1 flex-col gap-3 p-4 text-sm">
              <div>
                <p className="text-xs font-semibold text-slate-500 dark:text-slate-400">できるようになること</p>
                <p className="mt-1 leading-relaxed text-slate-700 dark:text-slate-200">{s.gain}</p>
              </div>
              <div className="mt-auto rounded-lg bg-slate-50 p-3 dark:bg-slate-800/60">
                <p className="text-xs font-semibold text-slate-500 dark:text-slate-400">実装するもの</p>
                <ul className="mt-1 list-disc space-y-0.5 pl-4 text-slate-700 dark:text-slate-300">
                  {s.build.map((b) => (
                    <li key={b}>{b}</li>
                  ))}
                </ul>
              </div>
            </div>
          </li>
        ))}
      </ol>
      <figcaption className="mt-3 text-center text-sm text-slate-500 dark:text-slate-400">
        一度に全部を実装する必要はありません。段階 1 だけでも、利用者の記録を守れます。
      </figcaption>
    </figure>
  );
}
