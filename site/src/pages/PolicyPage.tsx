import { Ban, EyeOff, FileCheck2, HandCoins, Scale, ShieldCheck } from "lucide-react";
import { C, Callout, Card, CardGrid, DataTable, ExtLink, PageHeader, Prose, Section, Toc } from "../components/ui";
import { DOCS } from "../content/links";

const TOC = [
  { id: "summary", label: "要点" },
  { id: "trademark", label: "商標・著作物" },
  { id: "nonprofit", label: "非営利" },
  { id: "privacy", label: "個人情報" },
  { id: "records", label: "対戦記録" },
  { id: "covenant", label: "運用者の約束事" },
  { id: "license", label: "ライセンス" },
];

export function PolicyPage() {
  return (
    <>
      <PageHeader
        eyebrow="ポリシー"
        title="守り続けるための約束"
        lead={
          <>
            Smash-hub の開発プロジェクトと、連合に参加するすべてのインスタンスに関わる方針の要約です。正式な内容は{" "}
            <ExtLink href={DOCS.policy}>運営ポリシー</ExtLink> と <ExtLink href={DOCS.covenant}>運用者の約束事</ExtLink> にあります。
          </>
        }
      />
      <Prose>
        <div className="py-8">
          <Toc items={TOC} />
        </div>

        <Section id="summary" title="1. 要点">
          <CardGrid>
            <Card icon={Ban} title="公式の素材を使わない">
              任天堂の利用許可が出ている素材以外は使いません。ファイターやステージはテキストだけで扱います。
            </Card>
            <Card icon={HandCoins} title="プロジェクトは収益を得ない">
              ソフトウェアに広告・課金・アフィリエイト・トラッキングを組み込みません。
            </Card>
            <Card icon={EyeOff} title="個人情報を持たない">
              メールアドレス・パスワード・外部サービスのアカウント名を受け取らず、保存しません。
            </Card>
            <Card icon={ShieldCheck} title="記録を改ざんしない">
              確定した記録は更新・削除せず、取り消しも新しいイベントの追記で表します。
            </Card>
            <Card icon={FileCheck2} title="記録の公開に同意を得る">
              対戦記録は公開され、連合先に複製されます。登録時にこの点を明示して同意を得ます。
            </Card>
            <Card icon={Scale} title="AGPL-3.0 で公開する">
              改変して公開サーバーで動かす場合も、利用者にソースコードを提供します。
            </Card>
          </CardGrid>
        </Section>

        <Section id="trademark" title="2. 商標・著作物">
          <p>
            「大乱闘スマッシュブラザーズ」は任天堂株式会社の登録商標です。ゲームに関する著作権・商標権は任天堂株式会社および関連する権利者に帰属します。
            Smash-hub は有志による非公式のファンプロジェクトで、任天堂株式会社とは一切関係ありません。
          </p>
          <DataTable
            head={["使わないもの", "代わりにどうするか"]}
            rows={[
              ["ゲーム画面のスクリーンショット・動画からの切り出し", "使わない（通報の証拠は運営者だけが見る一時保管とし、対応後に削除）"],
              ["ファイター・ステージの画像・アイコン・シルエット", <>テキストで表す（記録では <C>mario</C> のような ID）</>],
              ["公式ロゴ・シリーズマーク・フォント・効果音・BGM", "使わない。公式の意匠に寄せないデザインにする"],
              ["上記を模写・トレースした素材", "使わない"],
            ]}
          />
          <p>権利者から申し出があった場合は、その内容に従い速やかに対応します。</p>
        </Section>

        <Section id="nonprofit" title="3. 非営利（開発プロジェクト自身の方針）">
          <p>
            Smash-hub プロジェクトはサイト運営で収益を得ません。広告・有料機能・課金・アフィリエイト・データの販売を行わず、寄付も当面は受け付けません。
          </p>
          <Callout kind="note" title="連合に参加する他のサイトについて">
            <p>
              連合には <strong>営利・非営利を問わず</strong> 参加できます。各運営者の営利活動や、それによる権利者の権利の侵害について、Smash-hub プロジェクトおよび連合プロトコルは責任を負いません。
              広告を掲載する場合も、利用者を追跡しない形にすることが参加の条件です。
            </p>
          </Callout>
        </Section>

        <Section id="privacy" title="4. 個人情報">
          <ul className="list-disc space-y-2 pl-6">
            <li>
              ログインは外部の認証プロバイダー（Discord / Google）に任せ、パスワード認証は実装しません。インスタンスには、プロバイダーの利用者 ID を元に戻せない形でハッシュ化した値だけを保存します。
            </li>
            <li>連合プロトコルは個人情報を取り扱いません。連合に出るのは、アカウント ID・表示名・確定した対戦記録だけです。</li>
            <li>アクセスログ（IP アドレスを含む）は不正対策のためだけに短期間（初期値 14日）保持し、連合では共有しません。</li>
            <li>利用者を追跡するアクセス解析・トラッキング・広告タグを設置しません。このサイトも外部のフォントやスクリプトを読み込みません。</li>
          </ul>
          <p>
            詳しくは <ExtLink href={DOCS.dataProtection}>データ保護</ExtLink> を参照してください。
          </p>
        </Section>

        <Section id="records" title="5. 対戦記録">
          <ul className="list-disc space-y-2 pl-6">
            <li>確定した対戦記録は公開され、連合する複数のサーバーに複製されます。記録を失わないための中核的な仕組みです。</li>
            <li>プレイヤーは自分の対戦履歴をいつでもエクスポートできます。</li>
            <li>
              アカウントを削除すると、プロフィールを削除し、記録上の識別子を仮名化します。対戦相手のレートの整合性を保つため、記録そのものは仮名化された状態で残ります。
            </li>
          </ul>
        </Section>

        <Section id="covenant" title="6. 運用者の約束事">
          <p>連合に参加するインスタンスは、次のことに同意します。</p>
          <ul className="list-disc space-y-1 pl-6">
            <li>任天堂の利用許可が出ている素材以外を使わない。非公式のファンサイトであることを表示する。</li>
            <li>対戦記録を改ざんしない。架空のアカウント・対戦を作らない。</li>
            <li>利用者の個人情報を連合で共有しない。トラッキングを設置しない。</li>
            <li>自サイトのログを連合先に提供し続ける。閉鎖するときは、可能な限り 30日以上前に予告する。</li>
            <li>利用者が求めた場合、対戦履歴のエクスポートとアカウント移行を妨げない。</li>
          </ul>
          <p>
            全文は <ExtLink href={DOCS.covenant}>運用者の約束事</ExtLink> にあります。
          </p>
        </Section>

        <Section id="license" title="7. ライセンス">
          <p>
            リポジトリ全体（このサイトを含む）は <ExtLink href={DOCS.license}>GNU Affero General Public License v3.0（AGPL-3.0）</ExtLink> で公開しています。
            改変したソフトウェアをネットワーク越しに提供する場合は、利用者にそのソースコードを提供してください。
          </p>
          <Callout kind="warning" title="法的助言ではありません">
            <p>このページは方針の要約で、法的助言ではありません。判断に迷う場合は「使わない・やらない」を選び、Issue で相談してください。</p>
          </Callout>
        </Section>
      </Prose>
    </>
  );
}
