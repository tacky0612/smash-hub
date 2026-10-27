import { MatchRecordSigning, RatingDeterminism, Resilience, ShareBoundary, SignedLogChain } from "../components/diagrams/ProtocolDiagrams";
import { SequenceDiagram, type Message, type Participant } from "../components/diagrams/SequenceDiagram";
import { StepFlow } from "../components/diagrams/StepFlow";
import { C, Callout, DataTable, ExtLink, PageHeader, Prose, Section, Toc } from "../components/ui";
import { DOCS } from "../content/links";

const TOC = [
  { id: "terms", label: "登場するもの" },
  { id: "log", label: "署名付きの追記専用ログ" },
  { id: "record", label: "対戦記録ができるまで" },
  { id: "sync", label: "pull で同期する" },
  { id: "rating", label: "どこでも同じになるレート" },
  { id: "cross", label: "インスタンスをまたぐ対戦" },
  { id: "resilience", label: "閉鎖しても記録が残る" },
  { id: "privacy", label: "共有しないもの" },
];

// インスタンスをまたぐ対戦の流れ（docs/architecture/federation.md の 11.4・11.5）
const CROSS_PARTICIPANTS: Participant[] = [
  { id: "太郎", lines: ["太郎", "インスタンス A の利用者"], tone: "muted" },
  { id: "A", lines: ["インスタンス A", "ホスト（先に待った側）"], tone: "primary" },
  { id: "B", lines: ["インスタンス B", "花子のホーム"], tone: "primary" },
  { id: "花子", lines: ["花子", "インスタンス B の利用者"], tone: "muted" },
];

const CROSS_MESSAGES: Message[] = [
  { section: "マッチング" },
  { from: "太郎", to: "A", label: "対戦相手を探す" },
  { from: "A", to: "B", label: "募集を配信（30秒で失効）", dashed: true },
  { from: "花子", to: "B", label: "対戦相手を探す" },
  { from: "B", to: "A", label: "申し込み（match-claims）" },
  { from: "A", to: "B", label: "成立（match_id）", tone: "success" },
  { section: "対戦" },
  { from: "B", to: "A", label: "状態の取得（部屋 ID）" },
  { from: "花子", to: "B", label: "部屋 ID を見て入室" },
  { section: "結果の報告" },
  { from: "太郎", to: "A", label: "勝ち（太郎の鍵で署名）" },
  { from: "花子", to: "B", label: "負け" },
  { from: "B", to: "A", label: "花子の署名付き報告を中継" },
  { from: "A", to: "A", label: "一致 → 記録をログに追記" },
  { from: "B", to: "A", label: "ログを pull して取り込む", tone: "primary" },
];

export function HowItWorksPage() {
  return (
    <>
      <PageHeader
        eyebrow="仕組み"
        title="署名とハッシュチェーンで、記録を誰のものでもなくする"
        lead={
          <>
            SmashHubプロトコルが使う技術は <strong>Ed25519 署名・SHA-256・JSON の正規化・HTTP の pull</strong> だけです。
            ブロックチェーンや特別なミドルウェアは使いません。このページでは、記録が作られてから各インスタンスで同じレートになるまでを順に説明します。
          </>
        }
      />
      <Prose>
        <div className="py-8">
          <Toc items={TOC} />
        </div>

        <Section id="terms" title="1. 登場するもの">
          <DataTable
            head={["用語", "意味"]}
            rows={[
              [<strong>インスタンス</strong>, "連合に参加している1つのサイト（サーバー）。SmashHub のソフトウェアでも、プロトコルを独自に実装した既存のサイトでもかまいません。"],
              [<strong>ホームインスタンス</strong>, "利用者がアカウントを作り、ログインしているインスタンス。"],
              [<strong>アカウント ID</strong>, <>利用者ごとの Ed25519 公開鍵から作る識別子（<C>did:key:z6Mk…</C>）。連合の中では、サイトのユーザー ID ではなくこの ID で人を識別します。</>],
              [<strong>イベント</strong>, <>インスタンスが確定させた出来事。アカウントの公開（<C>AccountPublished</C>）、対戦記録（<C>MatchRecorded</C>）、記録の無効化（<C>MatchVoided</C>）があります。</>],
              [<strong>ログ</strong>, "イベントを連番で追記していく、インスタンスごとの記録の帳簿。書き換え・削除はせず、取り消しも新しいイベントの追記で表します。"],
              [<strong>SmashHubレート</strong>, "連合で共有された対戦記録から、全インスタンスが同じ手順で計算する1つのレート。"],
            ]}
          />
        </Section>

        <Section id="log" title="2. 署名付きの追記専用ログ">
          <p>
            各インスタンスは、確定した出来事をイベントとしてログに追記します。イベントには連番（<C>seq</C>）、直前のイベントのハッシュ（<C>prev</C>）、インスタンスの Ed25519 署名を付けます。
          </p>
          <SignedLogChain />
          <p>
            <C>AccountPublished</C> には利用者自身の鍵での署名も付くので、インスタンスが勝手に他人の表示名を書き換えることはできません。
          </p>
        </Section>

        <Section id="record" title="3. 対戦記録ができるまで">
          <p>
            対戦が終わると、2人のプレイヤーがそれぞれ結果を報告します。報告は「対戦結果の内容」への <strong>各プレイヤーの鍵での署名</strong> です。
            報告が一致すると、2人の署名がそろった <C>MatchRecorded</C> を、その対戦の <strong>ホスト</strong> がログに追記します。
          </p>
          <MatchRecordSigning />
          <ul className="list-disc space-y-1 pl-6">
            <li>ホストのインスタンスでも、片方のプレイヤーに無断で記録を作れません。</li>
            <li>
              内容の SHA-256 が <strong>記録 ID</strong> です。同じ対戦結果なら、どのインスタンスで計算しても同じ ID になります。
            </li>
            <li>記録に入るのはアカウント ID・ファイター ID・勝敗・ルール・日時だけで、表示名や画像は入りません。</li>
          </ul>
        </Section>

        <Section id="sync" title="4. pull で同期する">
          <p>他のインスタンスは、自分のタイミングで差分を取りに行きます。相手が止まっていても、復帰後に追いつけます。</p>
          <StepFlow
            title="ログの同期と検証の流れ"
            steps={[
              { title: "差分を取得", body: "最後に取り込んだ seq の続きを取りに行く。", tag: "GET /federation/log?since=" },
              { title: "インスタンス署名", body: "最初に取得して固定した公開鍵で、イベントの署名を検証する。" },
              { title: "連番とハッシュ", body: "seq が続いていて、prev が前のイベントのハッシュと一致するか確かめる。" },
              { title: "プレイヤーの署名", body: "対戦記録なら、両プレイヤーの署名をそれぞれのアカウント ID の公開鍵で検証する。" },
              { title: "取り込む", body: "検証に通ったものだけを保存し、レートを再計算する。失敗したらそこで止めて次回に再試行。" },
            ]}
          />
        </Section>

        <Section id="rating" title="5. どこでも同じになるレート">
          <p>
            レートはサイトの DB が持つ値ではなく、<strong>共有された対戦記録から決定的に再計算する派生データ</strong> です。
            記録を決まった順に並べ、整数演算だけのスマメイト式で1件ずつ適用するので、記録の集合が同じならどの言語で実装しても同じ値になります。
          </p>
          <RatingDeterminism />
          <Callout kind="note" title="決定性を守るための約束">
            <ul className="list-disc space-y-1 pl-5">
              <li>到着順・現在時刻・乱数・浮動小数点に結果を左右させない。</li>
              <li>ホストがログに追記した日時と対戦日時の差が7日を超える記録は、レート計算に入れない（後出しの記録でレートを動かさない）。</li>
              <li>
                詳しい計算式とテストベクタは <ExtLink href={DOCS.rating}>レーティング</ExtLink> にあります。
              </li>
            </ul>
          </Callout>
        </Section>

        <Section id="cross" title="6. インスタンスをまたぐ対戦">
          <p>
            待機中の対戦募集を連合先に配り合うので、自分のインスタンスに相手がいなくても、連合全体から相手が見つかります。
            先に待っていた側のインスタンスが <strong>ホスト</strong> になり、結果の受付と記録の作成を担当します。
          </p>
          <SequenceDiagram
            title="インスタンスをまたぐ対戦の流れ"
            caption="インスタンス間のリクエストには、送信元インスタンスの鍵で署名します。記録を作れるのはホストだけで、他のインスタンスから記録そのものを送り込む API はありません。"
            participants={CROSS_PARTICIPANTS}
            messages={CROSS_MESSAGES}
          />
        </Section>

        <Section id="resilience" title="7. 閉鎖しても記録が残る">
          <p>
            確定した記録は連合先に複製されます。インスタンスが閉鎖しても、利用者の戦績とレートは他のインスタンスに残り、署名によって本物かどうかを検証できます。
          </p>
          <Resilience />
          <p className="text-sm text-slate-500">
            ログの再配布・スナップショット・アカウント移行は今後実装します（
            <ExtLink href={DOCS.roadmap}>ロードマップ</ExtLink>）。
          </p>
        </Section>

        <Section id="privacy" title="8. 共有しないもの">
          <p>このプロトコルは個人情報を取り扱いません。連合に出るのは、公開鍵から作るアカウント ID・利用者が決めた表示名・確定した対戦記録だけです。</p>
          <ShareBoundary />
          <p>
            仕様の全体は <ExtLink href={DOCS.federation}>SmashHubプロトコル</ExtLink> にあります。
          </p>
        </Section>
      </Prose>
    </>
  );
}
