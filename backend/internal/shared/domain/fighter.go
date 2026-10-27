package domain

// FighterID はファイターの識別子。画像は持たず、テキストの識別子と表示名だけを扱う。
type FighterID string

// FighterRandom は「おまかせ」。
const FighterRandom FighterID = "random"

// Fighter はファイターの識別子と表示名。
type Fighter struct {
	ID   FighterID `json:"id"`
	Name string    `json:"name"`
}

// Fighters は使用できるファイターの一覧（参戦順）。ID は SmashHubプロトコルの対戦記録でも使う（対応表: docs/domain/fighters.md）。
// 一度割り当てた ID は変更・再利用しない。
var Fighters = []Fighter{
	{"mario", "マリオ"},
	{"donkey_kong", "ドンキーコング"},
	{"link", "リンク"},
	{"samus", "サムス"},
	{"dark_samus", "ダークサムス"},
	{"yoshi", "ヨッシー"},
	{"kirby", "カービィ"},
	{"fox", "フォックス"},
	{"pikachu", "ピカチュウ"},
	{"luigi", "ルイージ"},
	{"ness", "ネス"},
	{"captain_falcon", "キャプテン・ファルコン"},
	{"jigglypuff", "プリン"},
	{"peach", "ピーチ"},
	{"daisy", "デイジー"},
	{"bowser", "クッパ"},
	{"ice_climbers", "アイスクライマー"},
	{"sheik", "シーク"},
	{"zelda", "ゼルダ"},
	{"dr_mario", "ドクターマリオ"},
	{"pichu", "ピチュー"},
	{"falco", "ファルコ"},
	{"marth", "マルス"},
	{"lucina", "ルキナ"},
	{"young_link", "こどもリンク"},
	{"ganondorf", "ガノンドロフ"},
	{"mewtwo", "ミュウツー"},
	{"roy", "ロイ"},
	{"chrom", "クロム"},
	{"mr_game_and_watch", "Mr.ゲーム&ウォッチ"},
	{"meta_knight", "メタナイト"},
	{"pit", "ピット"},
	{"dark_pit", "ブラックピット"},
	{"zero_suit_samus", "ゼロスーツサムス"},
	{"wario", "ワリオ"},
	{"snake", "スネーク"},
	{"ike", "アイク"},
	{"pokemon_trainer", "ポケモントレーナー"},
	{"diddy_kong", "ディディーコング"},
	{"lucas", "リュカ"},
	{"sonic", "ソニック"},
	{"king_dedede", "デデデ"},
	{"olimar", "ピクミン&オリマー"},
	{"lucario", "ルカリオ"},
	{"rob", "ロボット"},
	{"toon_link", "トゥーンリンク"},
	{"wolf", "ウルフ"},
	{"villager", "むらびと"},
	{"mega_man", "ロックマン"},
	{"wii_fit_trainer", "Wii Fit トレーナー"},
	{"rosalina_and_luma", "ロゼッタ&チコ"},
	{"little_mac", "リトル・マック"},
	{"greninja", "ゲッコウガ"},
	{"mii_brawler", "Mii 格闘タイプ"},
	{"mii_swordfighter", "Mii 剣術タイプ"},
	{"mii_gunner", "Mii 射撃タイプ"},
	{"palutena", "パルテナ"},
	{"pac_man", "パックマン"},
	{"robin", "ルフレ"},
	{"shulk", "シュルク"},
	{"bowser_jr", "クッパJr."},
	{"duck_hunt", "ダックハント"},
	{"ryu", "リュウ"},
	{"ken", "ケン"},
	{"cloud", "クラウド"},
	{"corrin", "カムイ"},
	{"bayonetta", "ベヨネッタ"},
	{"inkling", "インクリング"},
	{"ridley", "リドリー"},
	{"simon", "シモン"},
	{"richter", "リヒター"},
	{"king_k_rool", "キングクルール"},
	{"isabelle", "しずえ"},
	{"incineroar", "ガオガエン"},
	{"piranha_plant", "パックンフラワー"},
	{"joker", "ジョーカー"},
	{"hero", "勇者"},
	{"banjo_and_kazooie", "バンジョー&カズーイ"},
	{"terry", "テリー"},
	{"byleth", "ベレト/ベレス"},
	{"min_min", "ミェンミェン"},
	{"steve", "スティーブ/アレックス"},
	{"sephiroth", "セフィロス"},
	{"pyra_mythra", "ホムラ/ヒカリ"},
	{"kazuya", "カズヤ"},
	{"sora", "ソラ"},
}

var fighterSet = func() map[FighterID]struct{} {
	m := make(map[FighterID]struct{}, len(Fighters))
	for _, f := range Fighters {
		m[f.ID] = struct{}{}
	}
	return m
}()

// IsValidFighter は id が使用できるファイターかを返す。allowRandom が true なら「おまかせ」も許可する。
func IsValidFighter(id FighterID, allowRandom bool) bool {
	if allowRandom && id == FighterRandom {
		return true
	}
	_, ok := fighterSet[id]
	return ok
}
