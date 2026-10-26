# ファイター ID

> ステータス: **Draft**。

連合プロトコルの対戦記録（`MatchRecorded` の `picks`）では、ファイターを **数値のファイター ID** で表します。表示名は記録に入れず、この対応表で表示します。

## 方針

- ファイター ID は **Smash-hub 独自の連番** です。公式の参戦番号とは一致しません（ダッシュファイター等も独立した番号を持ちます）。
- **一度割り当てた ID は変更・再利用しません。** 新しいファイターは末尾に追加します。対戦記録は署名後に変更できないため、ID の意味が変わると過去の記録が読めなくなるからです。
- `0` は「おまかせ」です。マッチングの入力（1戦目のファイターの選択）でだけ使い、対戦記録には実際に使ったファイターの ID を記録します。
- 表示名はテキストのみで扱い、画像・アイコンは使いません（[運営ポリシー](../policy.md)）。
- 「内部識別子」は現在のバックエンド実装で使っている文字列です（`backend/internal/shared/domain/fighter.go`）。

## 対応表

| ID | ファイター | 内部識別子 |
|---:|---|---|
| 0 | おまかせ（マッチングの入力のみ） | `random` |
| 1 | マリオ | `mario` |
| 2 | ドンキーコング | `donkey_kong` |
| 3 | リンク | `link` |
| 4 | サムス | `samus` |
| 5 | ダークサムス | `dark_samus` |
| 6 | ヨッシー | `yoshi` |
| 7 | カービィ | `kirby` |
| 8 | フォックス | `fox` |
| 9 | ピカチュウ | `pikachu` |
| 10 | ルイージ | `luigi` |
| 11 | ネス | `ness` |
| 12 | キャプテン・ファルコン | `captain_falcon` |
| 13 | プリン | `jigglypuff` |
| 14 | ピーチ | `peach` |
| 15 | デイジー | `daisy` |
| 16 | クッパ | `bowser` |
| 17 | アイスクライマー | `ice_climbers` |
| 18 | シーク | `sheik` |
| 19 | ゼルダ | `zelda` |
| 20 | ドクターマリオ | `dr_mario` |
| 21 | ピチュー | `pichu` |
| 22 | ファルコ | `falco` |
| 23 | マルス | `marth` |
| 24 | ルキナ | `lucina` |
| 25 | こどもリンク | `young_link` |
| 26 | ガノンドロフ | `ganondorf` |
| 27 | ミュウツー | `mewtwo` |
| 28 | ロイ | `roy` |
| 29 | クロム | `chrom` |
| 30 | Mr.ゲーム&ウォッチ | `mr_game_and_watch` |
| 31 | メタナイト | `meta_knight` |
| 32 | ピット | `pit` |
| 33 | ブラックピット | `dark_pit` |
| 34 | ゼロスーツサムス | `zero_suit_samus` |
| 35 | ワリオ | `wario` |
| 36 | スネーク | `snake` |
| 37 | アイク | `ike` |
| 38 | ポケモントレーナー | `pokemon_trainer` |
| 39 | ディディーコング | `diddy_kong` |
| 40 | リュカ | `lucas` |
| 41 | ソニック | `sonic` |
| 42 | デデデ | `king_dedede` |
| 43 | ピクミン&オリマー | `olimar` |
| 44 | ルカリオ | `lucario` |
| 45 | ロボット | `rob` |
| 46 | トゥーンリンク | `toon_link` |
| 47 | ウルフ | `wolf` |
| 48 | むらびと | `villager` |
| 49 | ロックマン | `mega_man` |
| 50 | Wii Fit トレーナー | `wii_fit_trainer` |
| 51 | ロゼッタ&チコ | `rosalina_and_luma` |
| 52 | リトル・マック | `little_mac` |
| 53 | ゲッコウガ | `greninja` |
| 54 | Mii 格闘タイプ | `mii_brawler` |
| 55 | Mii 剣術タイプ | `mii_swordfighter` |
| 56 | Mii 射撃タイプ | `mii_gunner` |
| 57 | パルテナ | `palutena` |
| 58 | パックマン | `pac_man` |
| 59 | ルフレ | `robin` |
| 60 | シュルク | `shulk` |
| 61 | クッパJr. | `bowser_jr` |
| 62 | ダックハント | `duck_hunt` |
| 63 | リュウ | `ryu` |
| 64 | ケン | `ken` |
| 65 | クラウド | `cloud` |
| 66 | カムイ | `corrin` |
| 67 | ベヨネッタ | `bayonetta` |
| 68 | インクリング | `inkling` |
| 69 | リドリー | `ridley` |
| 70 | シモン | `simon` |
| 71 | リヒター | `richter` |
| 72 | キングクルール | `king_k_rool` |
| 73 | しずえ | `isabelle` |
| 74 | ガオガエン | `incineroar` |
| 75 | パックンフラワー | `piranha_plant` |
| 76 | ジョーカー | `joker` |
| 77 | 勇者 | `hero` |
| 78 | バンジョー&カズーイ | `banjo_and_kazooie` |
| 79 | テリー | `terry` |
| 80 | ベレト/ベレス | `byleth` |
| 81 | ミェンミェン | `min_min` |
| 82 | スティーブ/アレックス | `steve` |
| 83 | セフィロス | `sephiroth` |
| 84 | ホムラ/ヒカリ | `pyra_mythra` |
| 85 | カズヤ | `kazuya` |
| 86 | ソラ | `sora` |
