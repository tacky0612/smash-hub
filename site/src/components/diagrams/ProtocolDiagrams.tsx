// SmashHubプロトコルの図。各ページから使う。内容は docs/architecture/federation.md に合わせる。
// どの図も PC 用（横に広い構成）とスマートフォン用（縦に並べる構成）の2つを持ち、Responsive で切り替える。
// 図の内容を変えるときは、両方の構成を揃えて直す。
import { Arrow, Caption, Connector, Diagram, Node, Region, Responsive } from "./kit";

const M = 340; // スマートフォン用の図の幅

/** インスタンスどうしの関係（記録の複製・募集の配信・SmashHubレート）。 */
export function FederationMap() {
  const title = "連合の全体像: 独立したインスタンスが、署名付きの対戦記録を複製し合う";
  const caption = "各インスタンスは独立して運営され、確定した対戦記録だけを互いに複製します。どこで見ても同じ記録から、同じレートが計算されます。";
  return (
    <Responsive
      desktop={
        <Diagram title={title} caption={caption} width={780} height={470}>
          <Node x={85} y={14} w={130} h={36} lines={["利用者"]} tone="muted" />
          <Node x={565} y={14} w={130} h={36} lines={["利用者"]} tone="muted" />
          <Node x={325} y={420} w={130} h={36} lines={["利用者"]} tone="muted" />
          <Arrow from={[150, 52]} to={[150, 86]} tone="muted" />
          <Arrow from={[630, 52]} to={[630, 86]} tone="muted" />
          <Arrow from={[390, 418]} to={[390, 382]} tone="muted" />

          <Node x={40} y={90} w={220} h={80} lines={["インスタンス A", "SmashHub を運営", "署名付きログ"]} tone="primary" />
          <Node x={520} y={90} w={220} h={80} lines={["インスタンス B", "SmashHub を運営", "署名付きログ"]} tone="primary" />
          <Node x={280} y={300} w={220} h={80} lines={["既存の対戦サイト C", "プロトコルを独自に実装", "署名付きログ"]} tone="accent" />

          <Arrow from={[262, 118]} to={[518, 118]} both label="対戦記録の複製（pull）" tone="primary" />
          <Arrow from={[262, 146]} to={[518, 146]} both dashed label="対戦募集の配信・申し込み" tone="neutral" labelOffset={[0, 12]} />
          <Arrow from={[150, 172]} to={[300, 298]} both label="記録の複製" tone="primary" labelOffset={[-46, 0]} />
          <Arrow from={[630, 172]} to={[480, 298]} both label="記録の複製" tone="primary" labelOffset={[46, 0]} />

          <Caption x={390} y={222} text="SmashHubレート：どのインスタンスでも同じ値" strong />
          <Caption x={390} y={244} text="UI・アカウント・運営方針はそれぞれの自由" />
        </Diagram>
      }
      mobile={
        <Diagram title={title} caption={caption} width={M} height={470} minWidth={0}>
          <Node x={20} y={10} w={250} h={70} lines={["インスタンス A", "SmashHub を運営", "署名付きログ"]} tone="primary" />
          <Node x={20} y={160} w={250} h={70} lines={["インスタンス B", "SmashHub を運営", "署名付きログ"]} tone="primary" />
          <Node x={20} y={310} w={250} h={70} lines={["既存の対戦サイト C", "プロトコルを独自に実装", "署名付きログ"]} tone="accent" />
          <Arrow from={[145, 82]} to={[145, 158]} both label="記録の複製・募集の配信" tone="primary" labelOffset={[0, 0]} />
          <Arrow from={[145, 232]} to={[145, 308]} both label="記録の複製・募集の配信" tone="primary" labelOffset={[0, 0]} />
          <Connector d="M272,45 C330,45 330,345 272,345" both tone="primary" dashed />
          <Caption x={170} y={412} text="SmashHubレート：どこでも同じ値" strong />
          <Caption x={170} y={436} text="UI・アカウント・運営方針はそれぞれの自由" />
        </Diagram>
      }
    />
  );
}

/** 署名付きの追記専用ログ（ハッシュチェーン）。 */
export function SignedLogChain() {
  const title = "署名付きの追記専用ログ: 各イベントが直前のイベントのハッシュを持つ";
  const caption = "Git のコミットの連なりに近い構造です。途中を書き換えるとハッシュが合わなくなり、欠落は seq の飛びで分かります。";
  const events = [
    { lines: ["seq 1", "AccountPublished", "太郎の公開情報", "署名: インスタンス・太郎"], tone: "neutral" as const },
    { lines: ["seq 2", "AccountPublished", "花子の公開情報", "署名: インスタンス・花子"], tone: "neutral" as const },
    { lines: ["seq 3", "MatchRecorded", "太郎 vs 花子の結果", "署名: インスタンス・両プレイヤー"], tone: "primary" as const },
  ];
  return (
    <Responsive
      desktop={
        <Diagram title={title} caption={caption} width={780} height={230}>
          {events.map((e, i) => (
            <Node key={i} x={20 + i * 270} y={30} w={200} h={110} lines={e.lines} tone={e.tone} />
          ))}
          <Arrow from={[288, 85]} to={[222, 85]} label="prev = h(1)" labelOffset={[0, -18]} />
          <Arrow from={[558, 85]} to={[492, 85]} label="prev = h(2)" labelOffset={[0, -18]} />
          <Caption x={390} y={175} text="1つでも書き換えると、それ以降の prev が合わなくなる（改ざんの検知）" strong />
          <Caption x={390} y={200} text="seq は 1 からの連番。飛んでいれば欠落が分かる" />
        </Diagram>
      }
      mobile={
        <Diagram title={title} caption={caption} width={M} height={470} minWidth={0}>
          {events.map((e, i) => (
            <Node key={i} x={30} y={10 + i * 140} w={280} h={96} lines={e.lines} tone={e.tone} />
          ))}
          <Arrow from={[170, 148]} to={[170, 108]} label="prev = h(1)" labelOffset={[0, 0]} />
          <Arrow from={[170, 288]} to={[170, 248]} label="prev = h(2)" labelOffset={[0, 0]} />
          <Caption x={170} y={418} text="書き換えると prev が合わなくなる" strong />
          <Caption x={170} y={442} text="seq が飛んでいれば欠落が分かる" />
        </Diagram>
      }
    />
  );
}

/** 対戦記録ができるまで（両プレイヤーの署名と記録 ID）。 */
export function MatchRecordSigning() {
  const title = "対戦記録ができるまで: 両プレイヤーが同じ内容に署名し、ホストが記録する";
  const caption = "「太郎の勝ち」と「花子の負け」は同じ内容への署名になります。報告が一致すると、2人の署名がそろった記録になります。";
  return (
    <Responsive
      desktop={
        <Diagram title={title} caption={caption} width={780} height={310}>
          <Node x={20} y={30} w={170} h={64} lines={["太郎の報告", "勝ち"]} tone="neutral" />
          <Node x={20} y={200} w={170} h={64} lines={["花子の報告", "負け"]} tone="neutral" />
          <Node x={270} y={100} w={230} h={100} lines={["対戦結果の内容", "参加者・ファイター・勝者", "ルール・日時", "（正規化した JSON）"]} tone="neutral" />
          <Arrow from={[192, 62]} to={[268, 120]} label="太郎の鍵で署名" labelOffset={[-8, -14]} />
          <Arrow from={[192, 232]} to={[268, 180]} label="花子の鍵で署名" labelOffset={[-8, 14]} />
          <Node x={580} y={100} w={180} h={100} lines={["MatchRecorded", "内容 + 2人の署名", "ホストのログに追記"]} tone="primary" />
          <Arrow from={[502, 140]} to={[578, 140]} label="報告が一致" tone="primary" labelOffset={[0, -14]} />
          <Node x={580} y={236} w={180} h={56} lines={["記録 ID", "内容の SHA-256"]} tone="accent" />
          <Arrow from={[460, 202]} to={[578, 262]} label="SHA-256" tone="accent" labelOffset={[-10, 10]} />
        </Diagram>
      }
      mobile={
        <Diagram title={title} caption={caption} width={M} height={520} minWidth={0}>
          <Node x={10} y={10} w={150} h={56} lines={["太郎の報告", "勝ち"]} tone="neutral" />
          <Node x={180} y={10} w={150} h={56} lines={["花子の報告", "負け"]} tone="neutral" />
          <Arrow from={[85, 68]} to={[120, 128]} label="太郎の鍵で署名" labelOffset={[-10, 0]} />
          <Arrow from={[255, 68]} to={[220, 128]} label="花子の鍵で署名" labelOffset={[10, 0]} />
          <Node x={30} y={130} w={280} h={90} lines={["対戦結果の内容", "参加者・ファイター・勝者", "ルール・日時（正規化した JSON）"]} tone="neutral" />
          <Arrow from={[170, 222]} to={[170, 290]} label="報告が一致" tone="primary" labelOffset={[0, 0]} />
          <Node x={30} y={292} w={280} h={84} lines={["MatchRecorded", "内容 + 2人の署名", "ホストのログに追記"]} tone="primary" />
          <Arrow from={[170, 378]} to={[170, 438]} label="内容の SHA-256" tone="accent" labelOffset={[0, 0]} />
          <Node x={30} y={440} w={280} h={60} lines={["記録 ID", "どこで計算しても同じ値"]} tone="accent" />
        </Diagram>
      }
    />
  );
}

/** レートの決定性（同じ記録から、どこでも同じ値）。 */
export function RatingDeterminism() {
  const title = "SmashHubレート: 同じ記録の集合から、どのインスタンスでも同じ値を計算する";
  const caption = "レートは DB に保存した値ではなく、共有された記録から再計算する派生データです。到着順・時刻・乱数・浮動小数点に左右されません。";
  const outputs = ["インスタンス A", "インスタンス B", "既存の対戦サイト C"];
  return (
    <Responsive
      desktop={
        <Diagram title={title} caption={caption} width={780} height={300}>
          <Node x={20} y={105} w={180} h={90} lines={["共有された対戦記録", "締め切り（7日）以内", "無効化を除く"]} tone="neutral" />
          <Node x={270} y={95} w={220} h={110} lines={["決まった順に適用", "played_at → 記録 ID の順", "スマメイト式", "整数演算のみ"]} tone="primary" />
          <Arrow from={[202, 150]} to={[268, 150]} />
          {outputs.map((name, i) => (
            <g key={name}>
              <Node x={580} y={20 + i * 100} w={180} h={60} lines={[name, "太郎 1516 / 花子 1484"]} tone="success" />
              <Arrow from={[492, 150]} to={[578, 50 + i * 100]} tone="success" />
            </g>
          ))}
        </Diagram>
      }
      mobile={
        <Diagram title={title} caption={caption} width={M} height={510} minWidth={0}>
          <Node x={30} y={10} w={280} h={76} lines={["共有された対戦記録", "締め切り（7日）以内", "無効化を除く"]} tone="neutral" />
          <Arrow from={[170, 88]} to={[170, 126]} />
          <Node x={30} y={128} w={280} h={100} lines={["決まった順に適用", "played_at → 記録 ID の順", "スマメイト式", "整数演算のみ"]} tone="primary" />
          <Arrow from={[170, 230]} to={[170, 268]} tone="success" />
          <Region x={20} y={272} w={300} h={226} label="どこで計算しても同じ値" tone="success" />
          {outputs.map((name, i) => (
            <Node key={name} x={40} y={302 + i * 64} w={260} h={54} lines={[name, "太郎 1516 / 花子 1484"]} tone="success" />
          ))}
        </Diagram>
      }
    />
  );
}

/** インスタンスが閉鎖しても記録が残る。 */
export function Resilience() {
  const title = "インスタンスが閉鎖しても、記録は連合先に残る";
  const caption = "記録は両プレイヤーの署名付きなので、複製された先でも本物かどうかを検証できます。";
  return (
    <Responsive
      desktop={
        <Diagram title={title} caption={caption} width={780} height={250}>
          <Node x={40} y={85} w={220} h={80} lines={["インスタンス A（閉鎖）", "ログの提供を終了"]} tone="danger" dashed />
          <Node x={540} y={20} w={200} h={80} lines={["インスタンス B", "A のログの複製を保持"]} tone="success" />
          <Node x={540} y={150} w={200} h={80} lines={["既存の対戦サイト C", "A のログの複製を保持"]} tone="success" />
          <Arrow from={[262, 110]} to={[538, 60]} dashed label="閉鎖前に複製済み" tone="success" labelOffset={[0, -10]} />
          <Arrow from={[262, 140]} to={[538, 190]} dashed label="閉鎖前に複製済み" tone="success" labelOffset={[0, 10]} />
          <Caption x={150} y={200} text="利用者の戦績とレートは消えない" />
        </Diagram>
      }
      mobile={
        <Diagram title={title} caption={caption} width={M} height={300} minWidth={0}>
          <Node x={40} y={10} w={260} h={70} lines={["インスタンス A（閉鎖）", "ログの提供を終了"]} tone="danger" dashed />
          <Arrow from={[120, 82]} to={[90, 168]} dashed tone="success" />
          <Arrow from={[220, 82]} to={[250, 168]} dashed tone="success" />
          <Caption x={170} y={124} text="閉鎖前に複製済み" />
          <Node x={10} y={170} w={155} h={70} lines={["インスタンス B", "A の複製を保持"]} tone="success" />
          <Node x={175} y={170} w={155} h={70} lines={["既存サイト C", "A の複製を保持"]} tone="success" />
          <Caption x={170} y={274} text="利用者の戦績とレートは消えない" strong />
        </Diagram>
      }
    />
  );
}

/** 共有するもの・共有しないもの。 */
export function ShareBoundary() {
  const title = "共有するのは公開してよい事実だけ。個人情報はプロトコルに入れる項目がない";
  const caption = "メールアドレス・パスワード・外部 SNS のアカウント・IP アドレスを入れる項目は、イベントにも API にもありません。";
  const privateItems = [
    ["ログイン方式・アカウント DB", "外部プロバイダーの情報など"],
    ["対戦ルームのメッセージ", "通報・ブロック・マッチング条件"],
    ["サイト独自のレート・統計", "アクセスログ（IP アドレス）"],
  ];
  const sharedItems = [
    ["アカウント ID・表示名", "公開鍵から作る did:key"],
    ["確定した対戦記録", "両プレイヤーの署名付き"],
    ["待機中の対戦募集", "30秒で失効する一時データ"],
  ];
  return (
    <Responsive
      desktop={
        <Diagram title={title} caption={caption} width={780} height={330}>
          <Region x={20} y={20} w={340} h={290} label="あなたのサイトの中（共有しない）" />
          <Region x={420} y={20} w={340} h={290} label="連合で共有する" tone="primary" />
          {privateItems.map((lines, i) => (
            <Node key={lines[0]} x={45} y={55 + i * 84} w={290} h={64} lines={lines} tone="muted" />
          ))}
          {sharedItems.map((lines, i) => (
            <Node key={lines[0]} x={445} y={55 + i * 84} w={290} h={64} lines={lines} tone="primary" />
          ))}
          <Arrow from={[362, 165]} to={[418, 165]} tone="primary" />
        </Diagram>
      }
      mobile={
        <Diagram title={title} caption={caption} width={M} height={590} minWidth={0}>
          <Region x={10} y={10} w={320} h={258} label="あなたのサイトの中（共有しない）" />
          {privateItems.map((lines, i) => (
            <Node key={lines[0]} x={25} y={40 + i * 74} w={290} h={60} lines={lines} tone="muted" />
          ))}
          <Arrow from={[170, 272]} to={[170, 318]} tone="primary" label="公開してよい事実だけ" labelOffset={[0, 0]} />
          <Region x={10} y={322} w={320} h={258} label="連合で共有する" tone="primary" />
          {sharedItems.map((lines, i) => (
            <Node key={lines[0]} x={25} y={352 + i * 74} w={290} h={60} lines={lines} tone="primary" />
          ))}
        </Diagram>
      }
    />
  );
}
