package domain

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// docs/domain/fighters.md の対応表と Fighters の並びが一致することを確認する。
// ファイター ID は Fighters の並び順（1 始まり）で決まり、対戦記録に使われるため、
// 並びを変えたり途中に追加したりすると過去の記録の意味が変わってしまう。
func TestFighters_MatchDocumentedIDs(t *testing.T) {
	data, err := os.ReadFile("../../../../docs/domain/fighters.md")
	if err != nil {
		t.Fatal(err)
	}
	rows := regexp.MustCompile("(?m)^\\| (\\d+) \\| ([^|]+) \\| `([a-z_]+)` \\|$").FindAllStringSubmatch(string(data), -1)
	documented := map[int][2]string{}
	for _, r := range rows {
		id, _ := strconv.Atoi(r[1])
		documented[id] = [2]string{r[3], strings.TrimSpace(r[2])}
	}
	if got := documented[0][0]; got != string(FighterRandom) {
		t.Errorf("ID 0 は %q であるべき: %q", FighterRandom, got)
	}
	if len(documented) != len(Fighters)+1 {
		t.Fatalf("対応表の行数 %d とファイター数 %d+1 が一致しない", len(documented), len(Fighters))
	}
	for i, f := range Fighters {
		d, ok := documented[i+1]
		if !ok || d[0] != string(f.ID) || d[1] != f.Name {
			t.Errorf("ID %d: 対応表 %v、実装 %s / %s", i+1, d, f.ID, f.Name)
		}
	}
}
