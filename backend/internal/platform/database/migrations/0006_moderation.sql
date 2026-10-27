-- 対戦記録の無効化（MatchVoided。docs/architecture/federation.md 5.2）。
-- 無効化は記録のホストだけが発行でき、ホストがログに追記した日時（voided_at）と played_at の差が
-- 7日以内のものだけをレート計算で有効にする。記録そのもの（match_records）は消さない。
CREATE TABLE match_voids (
    record_id text PRIMARY KEY,
    host      text NOT NULL,
    reason    text NOT NULL,
    voided_at timestamptz NOT NULL
);

-- 通報（docs/domain/match-rules.md の「10. 通報とモデレーション」）。連合では共有しない。
-- 内容は運営者だけが見る。対応完了から 30日で削除する（docs/security/data-protection.md）。
CREATE TABLE reports (
    id          text PRIMARY KEY,
    reporter_id text NOT NULL,
    target_id   text NOT NULL,
    match_host  text NOT NULL DEFAULT '',
    match_id    text NOT NULL DEFAULT '',
    category    text NOT NULL,
    body        text NOT NULL,
    status      text NOT NULL,
    resolution  text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL,
    resolved_at timestamptz
);
CREATE INDEX reports_status ON reports (status, created_at);
CREATE INDEX reports_reporter ON reports (reporter_id, created_at);

-- 処分（注意・イエローカード・レッドカード）。モデレーションの記録として残す（通報を削除した後も残る）。
-- レッドカードはレーティング対戦の利用停止で、ends_at が NULL なら無期限。
CREATE TABLE sanctions (
    id         text PRIMARY KEY,
    account_id text NOT NULL,
    kind       text NOT NULL,
    reason     text NOT NULL,
    note       text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    ends_at    timestamptz,
    lifted_at  timestamptz
);
CREATE INDEX sanctions_account ON sanctions (account_id, created_at DESC);

-- 登録時に同意した利用規約の版（本文の SHA-256 の先頭）と日時。
ALTER TABLE accounts
    ADD COLUMN terms_version   text NOT NULL DEFAULT '',
    ADD COLUMN terms_agreed_at timestamptz;
