import type { LucideIcon } from "lucide-react";

/** アイコンの見た目を揃える丸い背景付きの枠。アイコンは lucide-react を使い、絵文字は使わない。 */
export function IconBadge({ icon: Icon, tone = "primary" }: { icon: LucideIcon; tone?: "primary" | "accent" | "neutral" }) {
  const tones = {
    primary: "bg-indigo-100 text-indigo-600 dark:bg-indigo-950 dark:text-indigo-300",
    accent: "bg-amber-100 text-amber-600 dark:bg-amber-950 dark:text-amber-300",
    neutral: "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300",
  };
  return (
    <span className={`inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg ${tones[tone]}`}>
      <Icon className="h-5 w-5" strokeWidth={1.8} aria-hidden />
    </span>
  );
}
