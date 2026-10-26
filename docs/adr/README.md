# Architecture Decision Records (ADR)

Smash-hub の重要な設計判断を記録します。運営者や開発者が入れ替わっても「なぜそうなっているのか」を追えるようにするためです。

## 書き方

1. [`0000-template.md`](0000-template.md) をコピーし、`NNNN-短いタイトル.md` として作成する（番号は連番）。
2. ステータスを `Proposed` にして PR を出す。
3. レビュー・議論を経てマージする際に `Accepted` にする。
4. 後から判断を変える場合は、元の ADR を書き換えず、新しい ADR を作って元の ADR を `Superseded by NNNN` にする。

## 一覧

| 番号 | タイトル | ステータス |
|---|---|---|
| [0001](0001-record-architecture-decisions.md) | 設計判断を ADR で記録する | Accepted |
| [0002](0002-external-authentication-providers.md) | 認証に外部プロバイダー（Discord / Google）を使い、個人情報を保持しない | Accepted |
| [0003](0003-license-agpl-3.0.md) | ライセンスを AGPL-3.0 とする | Accepted |
| [0004](0004-rating-formula.md) | レーティングの計算式にスマメイト式を採用する | Accepted |
| [0005](0005-tech-stack.md) | フロントエンドを React（SPA）、バックエンドを Go、DB を PostgreSQL とする | Accepted |
| [0006](0006-custom-federation-protocol.md) | 対戦記録の連合は独自プロトコルで行う | Accepted |
| [0007](0007-federation-open-to-commercial-sites.md) | 連合への参加に非営利を求めない | Accepted |
| [0008](0008-account-id-did-key.md) | アカウント ID は Ed25519 公開鍵の did:key とする | Accepted |
| [0009](0009-deployment-vps-startup-script.md) | インスタンスは国内 VPS のスタートアップスクリプトで立ち上げる | Accepted |
