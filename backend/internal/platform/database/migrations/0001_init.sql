-- アカウント。個人情報は持たない（表示名・メインファイターは利用者が入力した公開情報）。
CREATE TABLE accounts (
    id           text PRIMARY KEY,
    display_name text NOT NULL,
    main_fighter text NOT NULL,
    created_at   timestamptz NOT NULL
);

-- 外部プロバイダーとの紐づけ。利用者 ID は HMAC した識別子ハッシュとしてのみ保持する。
CREATE TABLE external_identities (
    identifier_hash text PRIMARY KEY,
    provider        text NOT NULL,
    account_id      text NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    created_at      timestamptz NOT NULL
);
CREATE INDEX external_identities_account_id ON external_identities (account_id);

-- セッション。Cookie にはトークンを、DB にはそのハッシュを保存する。
CREATE TABLE sessions (
    token_hash      text PRIMARY KEY,
    provider        text NOT NULL,
    identifier_hash text NOT NULL,
    account_id      text REFERENCES accounts (id) ON DELETE CASCADE,
    created_at      timestamptz NOT NULL,
    expires_at      timestamptz NOT NULL
);

-- OAuth の state と PKCE の verifier（短時間だけ保持する）。
CREATE TABLE oauth_states (
    state      text PRIMARY KEY,
    verifier   text NOT NULL,
    created_at timestamptz NOT NULL
);

-- 対戦。
CREATE TABLE matches (
    id           text PRIMARY KEY,
    player1_id   text NOT NULL REFERENCES accounts (id),
    player2_id   text NOT NULL REFERENCES accounts (id),
    fighter1     text NOT NULL,
    fighter2     text NOT NULL,
    room_id      text NOT NULL DEFAULT '',
    status       text NOT NULL,
    report1      text NOT NULL DEFAULT '',
    report2      text NOT NULL DEFAULT '',
    winner_id    text REFERENCES accounts (id),
    played_at    timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL,
    resolved_at  timestamptz
);
CREATE INDEX matches_player1 ON matches (player1_id, played_at DESC);
CREATE INDEX matches_player2 ON matches (player2_id, played_at DESC);
CREATE INDEX matches_confirmed ON matches (played_at, id) WHERE status = 'confirmed';

-- 対戦募集（マッチング待ち）。
CREATE TABLE match_requests (
    id           text PRIMARY KEY,
    account_id   text NOT NULL REFERENCES accounts (id),
    fighter      text NOT NULL,
    rate_range   integer NOT NULL, -- 0 は「制限なし」
    rating       integer NOT NULL,
    status       text NOT NULL,
    match_id     text REFERENCES matches (id),
    created_at   timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX match_requests_one_waiting ON match_requests (account_id) WHERE status = 'waiting';

-- レート（対戦記録から計算される派生データ）。
CREATE TABLE ratings (
    season_id  text NOT NULL,
    account_id text NOT NULL REFERENCES accounts (id),
    rating     integer NOT NULL,
    matches    integer NOT NULL,
    wins       integer NOT NULL,
    losses     integer NOT NULL,
    PRIMARY KEY (season_id, account_id)
);
