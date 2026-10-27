import type { LucideIcon } from "lucide-react";

/** アイコンの見た目を揃える丸い背景付きの枠。アイコンは lucide-react を使い、絵文字は使わない。 */
export function IconBadge({ icon: Icon, tone = "primary" }: { icon: LucideIcon; tone?: "primary" | "accent" | "neutral" }) {
  const tones = {
    primary: "bg-red-100 text-red-600",
    accent: "bg-slate-900 text-white",
    neutral: "bg-slate-100 text-slate-600",
  };
  return (
    <span className={`inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg ${tones[tone]}`}>
      <Icon className="h-5 w-5" strokeWidth={1.8} aria-hidden />
    </span>
  );
}
