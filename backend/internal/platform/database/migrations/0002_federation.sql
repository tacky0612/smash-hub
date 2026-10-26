-- 連合（docs/architecture/federation.md）のためのテーブル。

-- アカウントの署名鍵。ID は公開鍵から導出した did:key。秘密鍵は鍵暗号化鍵で暗号化して保存する。
ALTER TABLE accounts ADD COLUMN public_key bytea;
ALTER TABLE accounts ADD COLUMN sealed_private_key bytea;

-- 対戦の参加者・レートには他インスタンスのアカウントも入るため、accounts への外部キーを外す。
ALTER TABLE matches DROP CONSTRAINT matches_player1_id_fkey;
ALTER TABLE matches DROP CONSTRAINT matches_player2_id_fkey;
ALTER TABLE matches DROP CONSTRAINT matches_winner_id_fkey;
ALTER TABLE ratings DROP CONSTRAINT ratings_account_id_fkey;
-- 他インスタンスがホストする対戦に参加した募集は、自インスタンスの matches にない対戦 ID を持つ。
ALTER TABLE match_requests DROP CONSTRAINT match_requests_match_id_fkey;

-- プレイヤー2 が他インスタンスの利用者の場合、そのホームインスタンス。報告の中継元の確認に使う。
ALTER TABLE matches ADD COLUMN player2_home text NOT NULL DEFAULT '';
-- 各プレイヤーの結果報告の署名（対戦結果の内容 MatchStatement に対する署名）。
ALTER TABLE matches ADD COLUMN sig1 text NOT NULL DEFAULT '';
ALTER TABLE matches ADD COLUMN sig2 text NOT NULL DEFAULT '';

-- 追記専用ログ。自インスタンスのイベント（origin = 自分）と、他インスタンスから取り込んだイベントを持つ。
-- body は署名済みイベントの正規化 JSON をそのまま保存する（更新・削除しない）。
CREATE TABLE federation_events (
    origin     text NOT NULL,
    seq        bigint NOT NULL,
    type       text NOT NULL,
    hash       text NOT NULL,
    body       text NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (origin, seq)
);

-- 連合先と、取り込み済みの位置。
CREATE TABLE federation_peers (
    url            text PRIMARY KEY,
    public_key     text NOT NULL DEFAULT '',
    last_seq       bigint NOT NULL DEFAULT 0,
    last_hash      text NOT NULL DEFAULT '',
    last_error     text NOT NULL DEFAULT '',
    last_synced_at timestamptz
);

-- 確定した対戦記録（自インスタンスがホストした記録と、取り込んだ記録）。レート計算の入力。
CREATE TABLE match_records (
    match_id   text PRIMARY KEY,
    host       text NOT NULL,
    played_at  timestamptz NOT NULL,
    logged_at  timestamptz NOT NULL,
    player1_id text NOT NULL,
    player2_id text NOT NULL,
    winner     integer NOT NULL,
    body       text NOT NULL
);
CREATE INDEX match_records_player1 ON match_records (player1_id, played_at DESC);
CREATE INDEX match_records_player2 ON match_records (player2_id, played_at DESC);

-- 他インスタンスのアカウントの公開情報（AccountPublished から作る）。
CREATE TABLE remote_accounts (
    account_id    text PRIMARY KEY,
    display_name  text NOT NULL,
    home_instance text NOT NULL,
    updated_at    timestamptz NOT NULL
);

-- 他インスタンスから配信された対戦募集（一時データ）。
CREATE TABLE remote_match_requests (
    id         text PRIMARY KEY,
    origin     text NOT NULL,
    account_id text NOT NULL,
    fighter    text NOT NULL,
    rate_range integer NOT NULL,
    rating     integer NOT NULL,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL
);

-- 自インスタンスの利用者が参加している、他インスタンスがホストする対戦。
CREATE TABLE remote_matches (
    match_id   text PRIMARY KEY,
    host       text NOT NULL,
    account_id text NOT NULL,
    status     text NOT NULL,
    created_at timestamptz NOT NULL
);
CREATE INDEX remote_matches_account ON remote_matches (account_id, created_at DESC);
