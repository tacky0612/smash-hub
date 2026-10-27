// ページを組み立てる共通の部品。見た目の調整はここで行い、各ページは内容に集中する。
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Link } from "react-router";
import { IconBadge } from "./IconBadge";

/** ページ冒頭の見出し。 */
export function PageHeader({ eyebrow, title, lead }: { eyebrow: string; title: string; lead: ReactNode }) {
  return (
    <div className="border-b border-slate-200 bg-white">
      <div className="mx-auto max-w-4xl px-4 py-14 sm:px-6">
        <p className="flex items-center gap-3 text-sm font-semibold tracking-wide text-red-600">
          <span aria-hidden className="h-1 w-8 rounded-full bg-red-600" />
          {eyebrow}
        </p>
        <h1 className="mt-2 text-3xl font-bold tracking-tight text-slate-900 sm:text-4xl">{title}</h1>
        <div className="mt-4 text-lg leading-relaxed text-slate-600">{lead}</div>
      </div>
    </div>
  );
}

/** 本文の区切り。id を付けると目次から飛べる。 */
export function Section({ id, title, children }: { id?: string; title: string; children: ReactNode }) {
  return (
    <section id={id} className="scroll-mt-24 py-10">
      <h2 className="text-2xl font-bold tracking-tight text-slate-900">{title}</h2>
      <div className="mt-4 space-y-4 leading-relaxed text-slate-700">{children}</div>
    </section>
  );
}

/** 本文の幅のコンテナ。 */
export function Prose({ children }: { children: ReactNode }) {
  return <div className="mx-auto max-w-4xl divide-y divide-slate-200 px-4 sm:px-6">{children}</div>;
}

export function H3({ children }: { children: ReactNode }) {
  return <h3 className="pt-4 text-lg font-semibold text-slate-900">{children}</h3>;
}

/** カード（特徴・課題などを並べる）。 */
export function Card({ title, children, icon, tone = "neutral" }: { title: string; children: ReactNode; icon?: LucideIcon; tone?: "neutral" | "primary" | "accent" }) {
  const tones = {
    neutral: "border-slate-200 bg-white",
    primary: "border-red-200 bg-red-50/60",
    accent: "border-slate-300 bg-slate-50",
  };
  return (
    <div className={`rounded-xl border p-5 shadow-sm ${tones[tone]}`}>
      <div className="flex items-center gap-3">
        {icon && <IconBadge icon={icon} tone={tone === "accent" ? "accent" : "primary"} />}
        <h3 className="font-semibold text-slate-900">{title}</h3>
      </div>
      <div className="mt-2 text-sm leading-relaxed text-slate-600">{children}</div>
    </div>
  );
}

export function CardGrid({ children, cols = 3 }: { children: ReactNode; cols?: 2 | 3 }) {
  return <div className={`grid gap-4 sm:grid-cols-2 ${cols === 3 ? "lg:grid-cols-3" : ""}`}>{children}</div>;
}

/** 注意・補足の囲み。 */
export function Callout({ kind = "note", title, children }: { kind?: "note" | "warning" | "important"; title?: string; children: ReactNode }) {
  const styles = {
    note: "border-slate-300 bg-slate-50 text-slate-800",
    warning: "border-amber-300 bg-amber-50 text-amber-950",
    important: "border-red-300 bg-red-50 text-red-950",
  };
  const labels = { note: "補足", warning: "注意", important: "重要" };
  return (
    <aside className={`rounded-lg border-l-4 p-4 text-sm leading-relaxed ${styles[kind]}`}>
      <p className="font-semibold">{title ?? labels[kind]}</p>
      <div className="mt-1 space-y-2">{children}</div>
    </aside>
  );
}

/** コードの例。 */
export function CodeBlock({ code, label }: { code: string; label?: string }) {
  return (
    <div className="overflow-hidden rounded-lg border border-slate-800 bg-slate-950">
      {label && <div className="border-b border-slate-800 px-4 py-2 text-xs font-medium text-slate-400">{label}</div>}
      <pre className="overflow-x-auto p-4 text-[13px] leading-relaxed text-slate-100">
        <code className="font-mono">{code.trim()}</code>
      </pre>
    </div>
  );
}

/** 表。スマートフォンでは行ごとのカード（見出し: 値）に並べ替える。 */
export function DataTable({ head, rows }: { head: string[]; rows: ReactNode[][] }) {
  return (
    <>
      <div className="hidden overflow-x-auto rounded-lg border border-slate-200 sm:block">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-50 text-slate-900">
            <tr>
              {head.map((h) => (
                <th key={h} className="px-4 py-3 font-semibold">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-200">
            {rows.map((row, i) => (
              <tr key={i} className="align-top">
                {row.map((cell, j) => (
                  <td key={j} className="px-4 py-3 text-slate-700">
                    {cell}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <ul className="space-y-3 sm:hidden">
        {rows.map((row, i) => (
          <li key={i} className="rounded-lg border border-slate-200 p-4 text-sm">
            {/* 1列目を見出しにし、残りを「列名: 値」で並べる */}
            <div className="font-semibold text-slate-900">{row[0]}</div>
            <dl className="mt-2 space-y-2">
              {row.slice(1).map((cell, j) => (
                <div key={j}>
                  <dt className="text-xs font-semibold text-slate-500">{head[j + 1]}</dt>
                  <dd className="mt-0.5 text-slate-700">{cell}</dd>
                </div>
              ))}
            </dl>
          </li>
        ))}
      </ul>
    </>
  );
}

/** ボタン風のリンク。to は サイト内、href は外部。 */
export function ButtonLink({ to, href, children, variant = "primary" }: { to?: string; href?: string; children: ReactNode; variant?: "primary" | "secondary" }) {
  const cls =
    variant === "primary"
      ? "bg-red-600 text-white hover:bg-red-700"
      : "border border-slate-300 bg-white text-slate-800 hover:border-red-300 hover:text-red-700";
  const className = `inline-flex items-center justify-center gap-2 rounded-lg px-5 py-3 text-sm font-semibold shadow-sm transition-colors ${cls}`;
  if (to) {
    return (
      <Link to={to} className={className}>
        {children}
      </Link>
    );
  }
  return (
    <a href={href} target="_blank" rel="noopener noreferrer" className={className}>
      {children}
    </a>
  );
}

/** 外部（GitHub）へのリンク。 */
export function ExtLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noopener noreferrer" className="prose-link">
      {children}
    </a>
  );
}

/** インラインのコード。 */
export function C({ children }: { children: ReactNode }) {
  return <code className="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-[0.9em] text-slate-800">{children}</code>;
}

/** ページ内の目次。 */
export function Toc({ items }: { items: { id: string; label: string }[] }) {
  return (
    <nav aria-label="目次" className="rounded-xl border border-slate-200 bg-slate-50 p-5 text-sm">
      <p className="font-semibold text-slate-900">このページの内容</p>
      <ol className="mt-2 grid gap-1 sm:grid-cols-2">
        {items.map((item, i) => (
          <li key={item.id}>
            <a href={`#${item.id}`} onClick={(e) => scrollToId(e, item.id)} className="prose-link">
              {i + 1}. {item.label}
            </a>
          </li>
        ))}
      </ol>
    </nav>
  );
}

// HashRouter ではハッシュがルートに使われるため、ページ内リンクはスクロールで実現する
function scrollToId(e: React.MouseEvent, id: string) {
  e.preventDefault();
  document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
}
