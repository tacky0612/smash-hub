-- アクセスログ（docs/security/data-protection.md）。IP アドレスを含むため、保持期間（初期値 14日）を過ぎた行は定期的に削除する。
-- 連合では共有しない。クエリ文字列（OAuth の認可コードなど）は記録しない。
CREATE TABLE access_logs (
    at          timestamptz NOT NULL,
    ip          text        NOT NULL,
    method      text        NOT NULL,
    path        text        NOT NULL,
    status      integer     NOT NULL,
    duration_ms integer     NOT NULL
);
CREATE INDEX access_logs_at ON access_logs (at);
