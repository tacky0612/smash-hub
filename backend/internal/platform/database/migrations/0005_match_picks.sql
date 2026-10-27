-- 結果報告で各プレイヤーが報告したゲームごとのファイター（任意）と、それを含めた対戦結果の内容への署名
-- （docs/adr/0010-game-picks-in-report.md）。入力しなかった場合は NULL。
ALTER TABLE matches
    ADD COLUMN picks1     jsonb,
    ADD COLUMN picks2     jsonb,
    ADD COLUMN picks_sig1 text NOT NULL DEFAULT '',
    ADD COLUMN picks_sig2 text NOT NULL DEFAULT '';
