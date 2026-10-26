package domain

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// docs/domain/fighters.md の対応表と Fighters が一致することを確認する。
// ファイター ID は対戦記録に使われるため、ドキュメントと実装がずれないようにする。
func TestFighters_MatchDocumentedIDs(t *testing.T) {
	data, err := os.ReadFile("../../../../docs/domain/fighters.md")
	if err != nil {
		t.Fatal(err)
	}
	rows := regexp.MustCompile("(?m)^\\| `([a-z_]+)` \\| ([^|]+) \\|$").FindAllStringSubmatch(string(data), -1)
	documented := map[FighterID]string{}
	for _, r := range rows {
		documented[FighterID(r[1])] = strings.TrimSpace(r[2])
	}
	if _, ok := documented[FighterRandom]; !ok {
		t.Errorf("対応表に %q（おまかせ）がない", FighterRandom)
	}
	if len(documented) != len(Fighters)+1 {
		t.Fatalf("対応表の行数 %d とファイター数 %d+1 が一致しない", len(documented), len(Fighters))
	}
	for _, f := range Fighters {
		if name, ok := documented[f.ID]; !ok || name != f.Name {
			t.Errorf("%s: 対応表 %q、実装 %q", f.ID, name, f.Name)
		}
	}
}
