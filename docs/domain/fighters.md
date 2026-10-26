# ファイター ID

> ステータス: **Draft**。

連合プロトコルの対戦記録（`MatchRecorded` の `picks`）では、ファイターを **ファイター ID**（`mario` のような英小文字の識別子）で表します。日本語の表示名は記録に入れず、この対応表で表示します。

## 方針

- ファイター ID は Smash-hub 独自の識別子です（英小文字と `_`）。記録を読んだときに、どのファイターか分かりやすいことを優先しています。
- **一度割り当てた ID は変更・再利用しません。** 新しいファイターは ID を追加します。対戦記録は署名後に変更できないため、ID の意味が変わると過去の記録が読めなくなるからです。
- `random` は「おまかせ」です。マッチングの入力（1戦目のファイターの選択）でだけ使い、対戦記録には実際に使ったファイターの ID を記録します。
- 表示名はテキストのみで扱い、画像・アイコンは使いません（[運営ポリシー](../policy.md)）。
- 実装上の定義は `backend/internal/shared/domain/fighter.go` です。この表と一致していることをテストで確認しています。

## 対応表

| ID | ファイター |
|---|---|
| `random` | おまかせ（マッチングの入力のみ） |
| `random` | おまかせ（マッチングの入力のみ） |
| `mario` | マリオ |
| `donkey_kong` | ドンキーコング |
| `link` | リンク |
| `samus` | サムス |
| `dark_samus` | ダークサムス |
| `yoshi` | ヨッシー |
| `kirby` | カービィ |
| `fox` | フォックス |
| `pikachu` | ピカチュウ |
| `luigi` | ルイージ |
| `ness` | ネス |
| `captain_falcon` | キャプテン・ファルコン |
| `jigglypuff` | プリン |
| `peach` | ピーチ |
| `daisy` | デイジー |
| `bowser` | クッパ |
| `ice_climbers` | アイスクライマー |
| `sheik` | シーク |
| `zelda` | ゼルダ |
| `dr_mario` | ドクターマリオ |
| `pichu` | ピチュー |
| `falco` | ファルコ |
| `marth` | マルス |
| `lucina` | ルキナ |
| `young_link` | こどもリンク |
| `ganondorf` | ガノンドロフ |
| `mewtwo` | ミュウツー |
| `roy` | ロイ |
| `chrom` | クロム |
| `mr_game_and_watch` | Mr.ゲーム&ウォッチ |
| `meta_knight` | メタナイト |
| `pit` | ピット |
| `dark_pit` | ブラックピット |
| `zero_suit_samus` | ゼロスーツサムス |
| `wario` | ワリオ |
| `snake` | スネーク |
| `ike` | アイク |
| `pokemon_trainer` | ポケモントレーナー |
| `diddy_kong` | ディディーコング |
| `lucas` | リュカ |
| `sonic` | ソニック |
| `king_dedede` | デデデ |
| `olimar` | ピクミン&オリマー |
| `lucario` | ルカリオ |
| `rob` | ロボット |
| `toon_link` | トゥーンリンク |
| `wolf` | ウルフ |
| `villager` | むらびと |
| `mega_man` | ロックマン |
| `wii_fit_trainer` | Wii Fit トレーナー |
| `rosalina_and_luma` | ロゼッタ&チコ |
| `little_mac` | リトル・マック |
| `greninja` | ゲッコウガ |
| `mii_brawler` | Mii 格闘タイプ |
| `mii_swordfighter` | Mii 剣術タイプ |
| `mii_gunner` | Mii 射撃タイプ |
| `palutena` | パルテナ |
| `pac_man` | パックマン |
| `robin` | ルフレ |
| `shulk` | シュルク |
| `bowser_jr` | クッパJr. |
| `duck_hunt` | ダックハント |
| `ryu` | リュウ |
| `ken` | ケン |
| `cloud` | クラウド |
| `corrin` | カムイ |
| `bayonetta` | ベヨネッタ |
| `inkling` | インクリング |
| `ridley` | リドリー |
| `simon` | シモン |
| `richter` | リヒター |
| `king_k_rool` | キングクルール |
| `isabelle` | しずえ |
| `incineroar` | ガオガエン |
| `piranha_plant` | パックンフラワー |
| `joker` | ジョーカー |
| `hero` | 勇者 |
| `banjo_and_kazooie` | バンジョー&カズーイ |
| `terry` | テリー |
| `byleth` | ベレト/ベレス |
| `min_min` | ミェンミェン |
| `steve` | スティーブ/アレックス |
| `sephiroth` | セフィロス |
| `pyra_mythra` | ホムラ/ヒカリ |
| `kazuya` | カズヤ |
| `sora` | ソラ |
