import { Archive, ArrowRight, BookOpen, Code, Database, Lock, Plug, Scale, Server, ShieldCheck, Users, type LucideIcon } from "lucide-react";
import { IconBadge } from "../components/IconBadge";
import { FederationMap } from "../components/diagrams/ProtocolDiagrams";
import { ButtonLink, Card, CardGrid } from "../components/ui";
import { DISCUSSIONS, DOCS, REPO } from "../content/links";

const PROBLEMS = [
  {
    icon: Users,
    title: "過疎でマッチングしない",
    problem: "利用者の少ない時間帯・サイトでは、募集しても相手が見つからない。人は大きなサイトに流れ、小さなサイトは続けられなくなる。",
    answer: "対戦募集をサイトどうしで配り合い、別のサイトの利用者ともマッチングする。",
  },
  {
    icon: Scale,
    title: "レートがサイトごとに分断",
    problem: "「A サイトの 1800」と「B サイトの 1800」は比べられない。レートが強さの証明として弱くなる。",
    answer: "同じ署名付きの記録から、同じ計算式で計算する。レートは連合全体で1つ（連合レート）。",
  },
  {
    icon: Archive,
    title: "閉鎖で記録が消える",
    problem: "運営をやめると、利用者の何年分もの戦績が消える。運営者は「やめられない」重圧を抱える。",
    answer: "確定した記録を複数のサイトへ複製する。署名付きなので、どこに置かれていても本物か検証できる。",
  },
];

const PRINCIPLES = [
  { icon: Database, title: "記録が主、サーバーは従", body: "対戦記録は特定のサーバーではなくコミュニティ全体の資産。サーバーは記録を保持・中継する器です。" },
  { icon: ShieldCheck, title: "誰でも検証できる", body: "記録は両プレイヤーの署名付き。レートは記録から決定的に再計算できるので、運営者を信頼しなくても確かめられます。" },
  { icon: Server, title: "小さく運用できる", body: "個人が安価な VPS 1台で運用できることを前提にしています。ブロックチェーンや特別なミドルウェアは使いません。" },
  { icon: Lock, title: "個人情報を扱わない", body: "プロトコルには、メールアドレス・パスワード・IP アドレスを入れる項目がありません。共有するのは公開してよい事実だけです。" },
];

export function HomePage() {
  return (
    <>
      {/* ヒーロー */}
      <section className="border-b border-slate-200 bg-gradient-to-b from-indigo-50 via-white to-white dark:border-slate-800 dark:from-indigo-950/50 dark:via-slate-950 dark:to-slate-950">
        <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6">
          <p className="inline-flex rounded-full border border-indigo-200 bg-white px-3 py-1 text-xs font-semibold text-indigo-700 dark:border-indigo-900 dark:bg-indigo-950 dark:text-indigo-300">
            分散（連合）型・非営利・OSS
          </p>
          <h1 className="mt-6 max-w-3xl text-[1.65rem] leading-snug font-bold tracking-tight text-slate-900 sm:text-5xl sm:leading-tight dark:text-white">
            {/* 句の途中で折り返さないよう、句ごとにまとめる */}
            <span className="inline-block">どのサイトで遊んでいても、</span>
            <span className="inline-block">同じ相手と戦い、</span>
            <span className="inline-block">同じ物差しで比べられる。</span>
          </h1>
          <p className="mt-6 max-w-2xl text-lg leading-relaxed text-slate-600 dark:text-slate-300">
            Smash-hub は、大乱闘スマッシュブラザーズ SPECIAL の対戦記録とレートを、複数の独立したサイトで共有するための
            <strong className="text-slate-900 dark:text-white">連合プロトコル</strong>
            と、その参照実装です。1つのサイトが閉鎖しても、記録とサービスが続きます。
          </p>
          <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:flex-wrap">
            <ButtonLink to="/how-it-works">仕組みを見る</ButtonLink>
            <ButtonLink to="/adoption" variant="secondary">
              既存のサイトに導入する
            </ButtonLink>
            <ButtonLink href={REPO} variant="secondary">
              GitHub で見る
            </ButtonLink>
          </div>
        </div>
      </section>

      {/* 課題と答え */}
      <section className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
        <h2 className="text-2xl font-bold tracking-tight text-slate-900 sm:text-3xl dark:text-white">大きなサイトがなくなったあとに起きること</h2>
        <p className="mt-3 max-w-3xl text-slate-600 dark:text-slate-300">
          プレイヤーが複数の代替サイトに分かれると、どのサイトも同じ問題にぶつかります。連合プロトコルは、この3つをサイトどうしの協力で解決します。
        </p>
        <div className="mt-8 grid gap-4 lg:grid-cols-3">
          {PROBLEMS.map((p) => (
            <div key={p.title} className="flex flex-col overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
              <div className="p-5">
                <div className="flex items-center gap-3">
                  <IconBadge icon={p.icon} tone="accent" />
                  <h3 className="font-semibold text-slate-900 dark:text-white">{p.title}</h3>
                </div>
                <p className="mt-3 text-sm leading-relaxed text-slate-600 dark:text-slate-300">{p.problem}</p>
              </div>
              <div className="mt-auto border-t border-indigo-100 bg-indigo-50/70 p-5 dark:border-indigo-900 dark:bg-indigo-950/40">
                <p className="text-xs font-semibold text-indigo-700 dark:text-indigo-300">連合の答え</p>
                <p className="mt-1 text-sm leading-relaxed text-slate-700 dark:text-slate-200">{p.answer}</p>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* 全体像 */}
      <section className="border-y border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40">
        <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
          <h2 className="text-2xl font-bold tracking-tight text-slate-900 sm:text-3xl dark:text-white">全体像</h2>
          <p className="mt-3 max-w-3xl text-slate-600 dark:text-slate-300">
            それぞれのサイト（連合では <strong>インスタンス</strong> と呼びます）は独立して運営されます。共有するのは
            「確定した対戦記録」と「アカウントの公開情報」だけ。画面・アカウント・運営方針は各サイトの自由です。
          </p>
          <FederationMap />
          <div className="flex justify-center">
            <ButtonLink to="/how-it-works" variant="secondary">
              仕組みを詳しく見る
              <ArrowRight className="h-4 w-4" aria-hidden />
            </ButtonLink>
          </div>
        </div>
      </section>

      {/* 設計原則 */}
      <section className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
        <h2 className="text-2xl font-bold tracking-tight text-slate-900 sm:text-3xl dark:text-white">大切にしていること</h2>
        <div className="mt-8">
          <CardGrid cols={2}>
            {PRINCIPLES.map((p) => (
              <Card key={p.title} title={p.title} icon={p.icon}>
                {p.body}
              </Card>
            ))}
          </CardGrid>
        </div>
      </section>

      {/* 読者別の入口 */}
      <section className="border-t border-slate-200 bg-gradient-to-b from-white to-indigo-50 dark:border-slate-800 dark:from-slate-950 dark:to-indigo-950/40">
        <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
          <h2 className="text-2xl font-bold tracking-tight text-slate-900 sm:text-3xl dark:text-white">あなたに合った入口から</h2>
          <div className="mt-8 grid gap-4 md:grid-cols-3">
            <Entry icon={BookOpen} title="仕組みを知りたい" body="署名付きのログ、pull での同期、どこでも同じになるレート、インスタンスをまたぐ対戦を図で説明します。" to="/how-it-works" cta="仕組み" />
            <Entry icon={Plug} title="対戦サイトを運営・開発している" body="今のサイトの UI・アカウントはそのままで参加できます。3段階の導入方法と参加の条件をまとめました。" to="/adoption" cta="既存サイトへの導入" />
            <Entry icon={Code} title="実装に参加したい・自分で実装したい" body="API・イベントの形式・検証の手順・コード例と、ローカルで2つのインスタンスを動かす方法です。" to="/developers" cta="開発者向け" />
          </div>
          <p className="mt-10 text-sm text-slate-600 dark:text-slate-300">
            プロジェクトは開発初期です。仕様への意見や参加の相談は{" "}
            <a className="prose-link" href={DISCUSSIONS} target="_blank" rel="noopener noreferrer">
              GitHub Discussions
            </a>{" "}
            へ。背景は{" "}
            <a className="prose-link" href={DOCS.vision} target="_blank" rel="noopener noreferrer">
              ビジョン
            </a>{" "}
            にまとめています。
          </p>
        </div>
      </section>
    </>
  );
}

function Entry({ icon, title, body, to, cta }: { icon: LucideIcon; title: string; body: string; to: string; cta: string }) {
  return (
    <div className="flex flex-col rounded-xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <IconBadge icon={icon} />
      <h3 className="mt-4 font-semibold text-slate-900 dark:text-white">{title}</h3>
      <p className="mt-2 flex-1 text-sm leading-relaxed text-slate-600 dark:text-slate-300">{body}</p>
      <div className="mt-5">
        <ButtonLink to={to} variant="secondary">
          {cta}
          <ArrowRight className="h-4 w-4" aria-hidden />
        </ButtonLink>
      </div>
    </div>
  );
}
