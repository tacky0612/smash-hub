import { Gauge, KeyRound, LayoutDashboard, Plus, Trophy } from "lucide-react";
import { AdoptionStages } from "../components/diagrams/AdoptionStages";
import { FederationMap, ShareBoundary } from "../components/diagrams/ProtocolDiagrams";
import { ButtonLink, C, Callout, Card, CardGrid, DataTable, ExtLink, PageHeader, Prose, Section, Toc } from "../components/ui";
import { DISCUSSIONS, DOCS } from "../content/links";

const TOC = [
  { id: "why", label: "参加すると得られるもの" },
  { id: "keep", label: "変わらないもの" },
  { id: "share", label: "共有するもの・しないもの" },
  { id: "stages", label: "3段階の導入" },
  { id: "checklist", label: "導入の手順" },
  { id: "conditions", label: "参加の条件" },
  { id: "faq", label: "よくある質問" },
];

const FAQ = [
  {
    q: "今のサイトのレートはどうなりますか？",
    a: "SmashHubレートとは別に、サイト独自のレートとして残せます。SmashHub のレートは「SmashHubレート」という名前で表示し、独自のレートと取り違えないようにしてください。SmashHubレートは新しいシーズンから全員 1500 で始まり、過去の対戦や他のサービスのレートは引き継ぎません。",
  },
  {
    q: "利用者のメールアドレスやパスワードを渡す必要がありますか？",
    a: "ありません。プロトコルには個人情報を入れる項目がありません。連合に出すのは、アカウント ID・表示名・ホームのサイト・確定した対戦記録だけです。",
  },
  {
    q: "利用者の秘密鍵はどこに置きますか？",
    a: "当面は、ホームのサイトが暗号化して預かります。暗号化の鍵は DB とは別に保管し、利用者に鍵の管理はさせません。",
  },
  {
    q: "サーバーの負荷はどのくらいですか？",
    a: "連合先ごとに、数秒〜数十秒おきの HTTP GET（差分のみ・最大 500 件）と署名の検証だけです。参照実装も、メモリ 1GB の VPS 1台で動かすことを前提にしています。",
  },
  {
    q: "悪意のあるサイトが偽の記録を流したら？",
    a: "対戦記録には両プレイヤーの署名が必要なので、他のサイトの利用者を勝手に巻き込んだ記録は作れません。自サイトの架空のアカウントで記録を作る不正は、各インスタンスが拒否リストで連合から外します。",
  },
  {
    q: "SmashHub のソフトウェアに乗り換える必要がありますか？",
    a: "ありません。プロトコルを自分のサイトに実装すれば参加できます。一から作る場合は、SmashHub のインスタンスを立てて運営することもできます。",
  },
];

export function AdoptionPage() {
  return (
    <>
      <PageHeader
        eyebrow="既存の対戦サイトへの導入"
        title="今のサイトのまま、連合に参加する"
        lead={
          <>
            スマブラ SP の対戦募集サイト・レーティングサイトを運営・開発している方へ。共有するのは「確定した対戦記録」と「アカウントの公開情報」だけで、
            <strong>UI・アカウント・運営方針はそのまま</strong> です。営利・非営利を問わず参加できます。
          </>
        }
      />
      <Prose>
        <div className="py-8">
          <Toc items={TOC} />
        </div>

        <Section id="why" title="1. 参加すると得られるもの">
          <CardGrid cols={2}>
            <Card icon={Trophy} title="連合全体の利用者とマッチング">
              待機中の募集を連合先に配信し、他のサイトの募集にも申し込めます。自サイトに相手がいなくても、連合全体から相手が見つかります。
            </Card>
            <Card icon={Gauge} title="サイトをまたいで比べられるレート">
              全参加サイトが同じ記録から同じ計算式で計算するので、どのサイトで見ても同じレート・同じランキングになります。
            </Card>
            <Card icon={KeyRound} title="運営者を信頼しなくてよい記録">
              記録は両者の署名とハッシュチェーンで守られ、改ざんや欠落を誰でも検知できます。「レートが操作されている」と言われる余地が減ります。
            </Card>
            <Card icon={LayoutDashboard} title="閉鎖しても記録が残る">
              確定した記録は連合先に複製されます。サイトをやめても、利用者の何年分もの戦績は消えません。
            </Card>
          </CardGrid>
          <FederationMap />
        </Section>

        <Section id="keep" title="2. 変わらないもの">
          <ul className="list-disc space-y-2 pl-6">
            <li>
              <strong>ログイン方式とアカウント DB</strong> — 認証はこれまでどおりサイトの中で行います。
            </li>
            <li>
              <strong>サイト独自の機能</strong> — 大会・コミュニティ・配信連携など、連合に関係しない機能はそのまま使えます。
            </li>
            <li>
              <strong>サイト独自のレート</strong> — 併用できます。SmashHub のレートは「SmashHubレート」という名前で表示し、サイト独自のレートと区別できるようにしてください。
            </li>
          </ul>
        </Section>

        <Section id="share" title="3. 共有するもの・しないもの">
          <ShareBoundary />
          <p>サイトの中でどんな情報を保存し、どの認証方式を使うかは、各運営者の責任です。</p>
        </Section>

        <Section id="stages" title="4. 3段階の導入">
          <AdoptionStages />
        </Section>

        <Section id="checklist" title="5. 導入の手順">
          <DataTable
            head={["段階", "やること", "詳しくは"]}
            rows={[
              [
                "1",
                <>
                  利用者ごとに Ed25519 の鍵ペアを作り、アカウント ID（<C>did:key</C>）を割り当てる。秘密鍵は暗号化して保存する。
                </>,
                <ExtLink href={DOCS.forExistingSites}>入門 5.1</ExtLink>,
              ],
              ["1", <>表示名の登録・変更で <C>AccountPublished</C>、対戦の確定で両者の署名付きの <C>MatchRecorded</C> をログに追記する。</>, <ExtLink href={DOCS.forExistingSites}>入門 5.2</ExtLink>],
              ["1", <><C>GET /federation/instance</C> と <C>GET /federation/log</C> を公開する。イベントは追記したバイト列のまま返す。</>, <ExtLink href={DOCS.forExistingSites}>入門 5.3</ExtLink>],
              ["2", "連合先ごとに最後に取り込んだ seq とハッシュを持ち、定期的に pull して検証・取り込む。", <ExtLink href={DOCS.forExistingSites}>入門 5.4</ExtLink>],
              ["2", "取り込んだ記録と自サイトの記録を合わせて、決定的な手順で SmashHubレートを計算する。", <ExtLink href={DOCS.rating}>レーティング</ExtLink>],
              ["3", "インスタンス間リクエストに署名し、募集の配信・申し込み・状態の取得・報告の中継を実装する。", <ExtLink href={DOCS.federation}>SmashHubプロトコル 11</ExtLink>],
            ]}
          />
          <Callout kind="important" title="利用者の同意を得てください">
            <p>
              対戦記録は公開され、連合先に複製され、署名後は変更できません。既存の利用者を連合に参加させる前に、この点を説明して
              <strong>利用者ごとに同意を得てください</strong>。同意していない利用者は、これまでどおりサイトの中だけで遊べるようにしておくのが安全です。
            </p>
          </Callout>
          <Callout kind="note" title="連合に出せる対戦">
            <p>
              <ExtLink href={DOCS.matchRules}>標準ルール</ExtLink>（<C>standard-1on1@1</C>：2本先取・3ストック・7分）で行った対戦に限ります。サイト独自のルールの対戦は、連合に出さずにサイトの中だけで扱ってください。
            </p>
          </Callout>
        </Section>

        <Section id="conditions" title="6. 参加の条件">
          <p>
            連合に参加するには <ExtLink href={DOCS.covenant}>運用者の約束事</ExtLink> への同意が必要です。主な内容は次のとおりです。
          </p>
          <ul className="list-disc space-y-1 pl-6">
            <li>非公式のファンサイトであることを表示する。</li>
            <li>対戦記録を改ざんしない。架空のアカウント・対戦を作らない。</li>
            <li>利用者の個人情報を連合で共有しない。</li>
            <li>自サイトのログを連合先に提供し続ける。閉鎖するときは、可能な限り 7日以上前に予告する。</li>
            <li>利用者が求めた場合、対戦履歴のエクスポートとアカウント移行を妨げない。</li>
          </ul>
          <Callout kind="note" title="素材・アクセス解析・営利活動は各サイトの判断です">
            <p>
              任天堂の素材の扱い、アクセス解析の設置、広告などの営利活動は参加条件ではなく、各運営者の判断と責任です。それによる権利者の権利の侵害や法令違反について、SmashHub プロジェクトおよび SmashHubプロトコルは責任を負いません。
            </p>
            <p>SmashHub プロジェクト自身は、任天堂の利用許可が出ている素材以外を使わず、非営利で運用します（<ExtLink href={DOCS.policy}>運営ポリシー</ExtLink>）。</p>
          </Callout>
        </Section>

        <Section id="faq" title="7. よくある質問">
          <div className="divide-y divide-slate-200 rounded-xl border border-slate-200">
            {FAQ.map((f) => (
              <details key={f.q} className="group p-4">
                <summary className="flex cursor-pointer list-none items-center justify-between gap-4 font-medium text-slate-900">
                  {f.q}
                  <Plus aria-hidden className="h-5 w-5 shrink-0 text-slate-400 transition-transform group-open:rotate-45" />
                </summary>
                <p className="mt-3 text-sm leading-relaxed text-slate-600">{f.a}</p>
              </details>
            ))}
          </div>
          <div className="mt-8 flex flex-wrap gap-3">
            <ButtonLink href={DISCUSSIONS}>参加を相談する（Discussions）</ButtonLink>
            <ButtonLink to="/developers" variant="secondary">
              開発者向けの情報
            </ButtonLink>
          </div>
        </Section>
      </Prose>
    </>
  );
}
