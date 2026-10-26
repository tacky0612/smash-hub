-- 対戦記録を記録 ID（対戦結果の内容の正規化 JSON の SHA-256）で識別する（docs/architecture/federation.md 12.1）。
-- match_id はホストが決める値で連合全体では一意と限らないため、主キーを記録 ID に変える。
ALTER TABLE match_records ADD COLUMN record_id text;

-- 既存の記録: body は記録の正規化 JSON（キーは昇順・空白なし）なので、
-- signatures の項目を取り除くと、両プレイヤーが署名した対戦結果の内容のバイト列になる。
-- signatures の要素はアカウント ID と base64 の署名だけで、"]" を含まない。
UPDATE match_records
SET record_id = encode(sha256(convert_to(regexp_replace(body, ',"signatures":\[[^\]]*\]', ''), 'UTF8')), 'hex');

ALTER TABLE match_records DROP CONSTRAINT match_records_pkey;
ALTER TABLE match_records ALTER COLUMN record_id SET NOT NULL;
ALTER TABLE match_records ADD PRIMARY KEY (record_id);
CREATE INDEX match_records_played ON match_records (played_at, record_id);
