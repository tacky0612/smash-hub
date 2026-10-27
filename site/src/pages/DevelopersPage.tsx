import { Bug, GitPullRequest, MessagesSquare } from "lucide-react";
import { SequenceDiagram, type Message, type Participant } from "../components/diagrams/SequenceDiagram";
import { StepFlow } from "../components/diagrams/StepFlow";
import { C, Callout, Card, CardGrid, CodeBlock, DataTable, ExtLink, H3, PageHeader, Prose, Section, Toc } from "../components/ui";
import { DISCUSSIONS, DOCS, ISSUES } from "../content/links";

const TOC = [
  { id: "quickstart", label: "ローカルで動かす" },
  { id: "api", label: "API" },
  { id: "event", label: "イベントの形式" },
  { id: "verify", label: "検証の手順" },
  { id: "code", label: "実装の例（TypeScript）" },
  { id: "signing", label: "インスタンス間リクエストの署名" },
  { id: "rating", label: "レート計算の落とし穴" },
  { id: "contribute", label: "参加する" },
];

const EVENT_EXAMPLE = `
{
  "type": "MatchRecorded",
  "version": 1,
  "instance_id": "https://a.example",
  "seq": 3,
  "prev": "9f2c…（seq 2 のイベントの SHA-256）",
  "created_at": "2026-10-26T10:30:05.000Z",
  "payload": {
    "match_id": "7ecf762d86638e326bf3875b2ddf8550",
    "host_instance": "https://a.example",
    "played_at": "2026-10-26T10:10:00.000Z",
    "ruleset": "standard-1on1@1",
    "players": [
      { "account_id": "did:key:z6MkTaro…" },
      { "account_id": "did:key:z6MkHanako…" }
    ],
    "picks": [
      { "game": 1, "characters": ["mario", "fox"] }
    ],
    "winner": 0,
    "signatures": [
      { "account_id": "did:key:z6MkTaro…", "sig": "base64…" },
      { "account_id": "did:key:z6MkHanako…", "sig": "base64…" }
    ]
  },
  "sig": "base64…（インスタンスの署名）"
}`;

const CANONICAL = `
// キーを昇順に並べ、空白なしで出力する（署名とハッシュの対象）
export function canonicalize(v: unknown): string {
  if (Array.isArray(v)) return \`[\${v.map(canonicalize).join(",")}]\`;
  if (v !== null && typeof v === "object") {
    const obj = v as Record<string, unknown>;
    const keys = Object.keys(obj).filter((k) => obj[k] !== undefined).sort();
    return \`{\${keys.map((k) => \`\${JSON.stringify(k)}:\${canonicalize(obj[k])}\`).join(",")}}\`;
  }
  return JSON.stringify(v);
}`;

const VERIFY = `
import { createHash, createPublicKey, verify } from "node:crypto";

// Ed25519 の生の公開鍵（32 バイト）を KeyObject にする
const ed25519Key = (raw: Buffer) =>
  createPublicKey({
    key: Buffer.concat([Buffer.from("302a300506032b6570032100", "hex"), raw]),
    format: "der",
    type: "spki",
  });

// body は /federation/log の events の1要素（正規化 JSON の文字列）
export function verifyEvent(body: string, instanceKey: Buffer, lastSeq: number, lastHash: string) {
  const event = JSON.parse(body);
  const { sig, ...unsigned } = event; // 署名の対象は sig を除いたイベント
  const ok = verify(null, Buffer.from(canonicalize(unsigned)), ed25519Key(instanceKey), Buffer.from(sig, "base64"));
  if (!ok) throw new Error("instance signature is invalid");
  if (event.seq !== lastSeq + 1 || event.prev !== lastHash) throw new Error("hash chain is broken");
  const hash = createHash("sha256").update(body).digest("hex"); // 次のイベントの prev になる
  return { event, hash };
}`;

const DID = `
import bs58 from "bs58"; // base58btc（MIT ライセンス）

// did:key:z + base58btc(0xed 0x01 + 公開鍵 32 バイト)
export function publicKeyFromDid(did: string): Buffer {
  if (!did.startsWith("did:key:z")) throw new Error("not a did:key");
  const bytes = Buffer.from(bs58.decode(did.slice("did:key:z".length)));
  if (bytes[0] !== 0xed || bytes[1] !== 0x01 || bytes.length !== 34) throw new Error("not an Ed25519 did:key");
  return bytes.subarray(2);
}`;

const SIGNED_HEADERS = `
X-Smash-Hub-Instance:  送信元のインスタンス ID
X-Smash-Hub-Date:      送信日時（RFC 3339。受信側と 5分以上ずれたら拒否）
X-Smash-Hub-Signature: base64( Ed25519( "METHOD\\n/path?query\\n<日時>\\n<本文の SHA-256 の16進>" ) )`;

// 同期の流れ（受信側から見た pull）
const SYNC_PARTICIPANTS: Participant[] = [
  { id: "B", lines: ["あなたのインスタンス", "取り込む側"], tone: "primary" },
  { id: "A", lines: ["連合先のインスタンス", "ログを提供する側"], tone: "neutral" },
];
const SYNC_MESSAGES: Message[] = [
  { section: "初回だけ" },
  { from: "B", to: "A", label: "GET /federation/instance" },
  { from: "A", to: "B", label: "インスタンス ID・公開鍵", dashed: true },
  { from: "B", to: "B", label: "公開鍵を固定して保存" },
  { section: "数秒〜数十秒ごと" },
  { from: "B", to: "A", label: "GET /federation/log?since=最後の seq" },
  { from: "A", to: "B", label: "差分のイベント（最大 500 件）", dashed: true },
  { from: "B", to: "B", label: "検証して取り込み、レートを再計算" },
];

export function DevelopersPage() {
  return (
    <>
      <PageHeader
        eyebrow="開発者向け"
        title="プロトコルを実装する・参照実装に参加する"
        lead={
          <>
            参照実装は Go（バックエンド）と React（画面）で、PostgreSQL を使います。プロトコル自体は言語に依存しません。
            必要なのは Ed25519・SHA-256・JSON の正規化・HTTP だけです。
          </>
        }
      />
      <Prose>
        <div className="py-8">
          <Toc items={TOC} />
        </div>

        <Section id="quickstart" title="1. ローカルで動かす">
          <p>Docker があれば、連合した2つのインスタンスをすぐに起動できます。</p>
          <CodeBlock
            label="ターミナル"
            code={`
git clone https://github.com/tacky0612/smash-hub.git
cd smash-hub
make up          # Instance1: http://localhost:8080 / Instance2: http://localhost:8090
make test-e2e    # 2つのインスタンスで、連合での対戦の流れを検証する
make down`}
          />
          <p>
            開発用ログインで2人の利用者を作って対戦し、結果を報告すると、<C>http://localhost:8080/federation/log</C> に <C>AccountPublished</C> と{" "}
            <C>MatchRecorded</C> が並びます。このログを自分のコードで取得して検証できれば、取り込みの土台ができています。
          </p>
        </Section>

        <Section id="api" title="2. API">
          <DataTable
            head={["API", "認証", "内容"]}
            rows={[
              [<C>GET /federation/instance</C>, "なし", "インスタンス ID（連合先から到達できるベース URL）・名前・公開鍵（base64）"],
              [<C>GET /federation/log?since=&limit=</C>, "なし", "since の次の seq からのイベント（limit は既定 100・最大 500）"],
              [<C>POST /federation/match-requests</C>, "インスタンス署名", "待機中の対戦募集の配信（30秒で失効）"],
              [<C>POST /federation/match-claims</C>, "インスタンス署名", "配信された募集への申し込み"],
              [<C>GET /federation/matches/{"{id}"}</C>, "インスタンス署名", "進行中の対戦の状態（参加者のホームインスタンスだけ）"],
              [<C>POST /federation/match-reports</C>, "インスタンス署名", "結果報告（プレイヤーの署名付き）の中継"],
            ]}
          />
          <SequenceDiagram title="ログの pull 同期" participants={SYNC_PARTICIPANTS} messages={SYNC_MESSAGES} />
        </Section>

        <Section id="event" title="3. イベントの形式">
          <p>
            ログの1件は次の形です。署名とハッシュの対象は <strong>キーを昇順に並べ、空白を除いた JSON（正規化 JSON）</strong> です。
          </p>
          <CodeBlock label="MatchRecorded の例" code={EVENT_EXAMPLE} />
          <ul className="list-disc space-y-1 pl-6">
            <li>
              インスタンスの署名（<C>sig</C>）は、<C>sig</C> を除いたイベントの正規化 JSON に対する Ed25519 署名です。
            </li>
            <li>
              プレイヤーの署名は、<C>payload</C> から <C>signatures</C> を除いたもの（対戦結果の内容）の正規化 JSON に対する署名です。同じバイト列の SHA-256 が記録 ID です。
            </li>
            <li>最初のイベントは seq が 1、prev が空文字列です。時刻は UTC・ミリ秒までの RFC 3339 です。</li>
            <li>
              ファイターは <ExtLink href={DOCS.fighters}>ファイター ID</ExtLink>（<C>mario</C> 等）で表し、表示名や画像は入れません。
            </li>
          </ul>
        </Section>

        <Section id="verify" title="4. 検証の手順">
          <StepFlow
            title="受け取ったイベントの検証"
            steps={[
              { title: "インスタンス署名", body: "初回に固定した公開鍵で sig を検証する。" },
              { title: "連番とハッシュ", body: "seq が前回の続きで、prev が前回のイベントのハッシュと一致する。" },
              { title: "プレイヤーの署名", body: "MatchRecorded なら、両プレイヤーの署名をそれぞれの did:key の公開鍵で検証する。" },
              { title: "アカウントの署名", body: "AccountPublished なら、account_sig をそのアカウントの鍵で検証する。" },
              { title: "無効化の発行元", body: "MatchVoided なら、対象の記録を取り込み済みで、その記録のホストが発行元であることを確かめる。" },
              { title: "ルールセット", body: "standard-1on1@1 など、受け入れ可能なルールセットか確かめる。" },
            ]}
          />
          <p>検証に失敗したら、そのイベント以降を取り込まずに理由を記録し、次回に再試行します。</p>
        </Section>

        <Section id="code" title="5. 実装の例（TypeScript）">
          <p>
            Node.js の標準ライブラリ（<C>node:crypto</C>）だけで、署名・検証・ハッシュを扱えます。以下は考え方を示すための例です。参照実装は Go の{" "}
            <ExtLink href={DOCS.eventGo}>event.go</ExtLink> と <ExtLink href={DOCS.canonicalGo}>canonical.go</ExtLink> です。
          </p>
          <H3>正規化 JSON</H3>
          <CodeBlock label="canonicalize.ts" code={CANONICAL} />
          <Callout kind="warning">
            <p>
              シリアライズ形式の厳密な定義はまだ決まっていません。一部の文字（U+2028・U+2029・一部の制御文字）のエスケープは、<C>JSON.stringify</C> と Go の出力で異なります。
            </p>
            <p>受信したイベントは、自分で正規化し直さず、受け取ったバイト列のままハッシュと検証に使うのが安全です。</p>
          </Callout>
          <H3>インスタンス署名の検証とハッシュチェーン</H3>
          <CodeBlock label="verify-event.ts" code={VERIFY} />
          <H3>アカウント ID（did:key）から公開鍵を取り出す</H3>
          <CodeBlock label="did-key.ts" code={DID} />
        </Section>

        <Section id="signing" title="6. インスタンス間リクエストの署名">
          <p>
            <C>/federation/instance</C> と <C>/federation/log</C> 以外の API は、送信元インスタンスの鍵で署名したリクエストだけを受け付けます。連合先は許可リスト（<C>PEERS</C>）で限定します。
          </p>
          <CodeBlock label="リクエストヘッダー" code={SIGNED_HEADERS} />
        </Section>

        <Section id="rating" title="7. レート計算の落とし穴">
          <DataTable
            head={["落とし穴", "対策"]}
            rows={[
              ["浮動小数点の誤差", "整数演算だけで計算する。"],
              ["負の数の割り算", <>−∞ 方向に丸める（<C>floor_div</C>）。C 系の言語の整数除算は 0 方向に丸めるので補正が必要。</>],
              ["到着順に依存する", "played_at の順（同時刻なら記録 ID の昇順）で適用する。"],
              ["後から届いた記録", "ホストの追記日時と played_at の差が7日を超える記録と、ホストが対戦から7日以内に無効化した記録を除いてから計算する。"],
            ]}
          />
          <p>
            計算式とテストベクタは <ExtLink href={DOCS.rating}>レーティング</ExtLink> にあります。
          </p>
        </Section>

        <Section id="contribute" title="8. 参加する">
          <CardGrid>
            <Card icon={MessagesSquare} title="仕様について話す">
              「この API が足りない」「この形式では実装しにくい」といった声を歓迎します。<ExtLink href={DISCUSSIONS}>Discussions</ExtLink> へどうぞ。
            </Card>
            <Card icon={Bug} title="Issue を見る">
              <ExtLink href={ISSUES}>good first issue</ExtLink> から始められるものもあります。
            </Card>
            <Card icon={GitPullRequest} title="PR を送る">
              手順と規約は <ExtLink href={DOCS.contributing}>CONTRIBUTING.md</ExtLink> に、設計は <ExtLink href={DOCS.backend}>backend/README.md</ExtLink> にあります。
            </Card>
          </CardGrid>
          <Callout kind="note" title="現在の状況">
            <p>
              実装済み: アカウントと対戦記録のログ・記録の無効化・pull 同期と検証・SmashHubレート・インスタンスをまたぐ対戦。未実装: ログの再配布・スナップショット・鍵の更新・アカウント移行。ワイヤーフォーマットは変わる可能性があります（
              <ExtLink href={DOCS.roadmap}>ロードマップ</ExtLink>）。
            </p>
          </Callout>
        </Section>
      </Prose>
    </>
  );
}
