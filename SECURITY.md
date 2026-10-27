# セキュリティ

## 脆弱性の報告

SmashHub のソフトウェアに脆弱性を見つけた場合は、**公開の Issue・Discussion・SNS には書かず**、GitHub の [Private vulnerability reporting](https://docs.github.com/ja/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability) から非公開で報告してください（リポジトリの **Security** タブ → **Report a vulnerability**）。

報告には、可能な範囲で次の情報を含めてください。

- 影響を受けるコンポーネント（frontend / backend / SmashHubプロトコル / ドキュメント上の設計）
- 再現手順
- 想定される影響
- 修正案（あれば）

## 対応の流れ

| 段階 | 目安 |
|---|---|
| 受付の返信 | 3日以内 |
| トリアージ完了 | 7日以内 |
| 修正と公開 | 深刻度に応じて。公開は報告から最長 90日 |

修正版のリリース後、GitHub Security Advisory で公開します。報告者の希望に応じてクレジットを記載します。

## 対象

- このリポジトリのソースコード
- 設計ドキュメント上のプロトコル・データ設計の欠陥

## 対象外

- 個別のインスタンスの運用上の問題（各インスタンスの運営者に連絡してください）
- 外部認証プロバイダー（Discord・Google）自体の脆弱性
- 実在のインスタンスへの攻撃・負荷試験（**行わないでください**。検証は自分で立てた環境で行ってください）

## サポートするバージョン

現在は設計段階で、リリースされたバージョンはありません。リリース後は最新版のみをサポートします。

## 関連ドキュメント

セキュリティの方針・設計は [docs/security/](docs/security/README.md) を参照してください。
