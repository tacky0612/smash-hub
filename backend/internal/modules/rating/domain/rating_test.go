package domain

import (
	"encoding/csv"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// testVectorsFile は docs/domain/rating.md の「2.4 テストベクタ」の CSV（他の言語の実装でも使えるよう docs/ に置く）。
const testVectorsFile = "../../../../../docs/domain/rating-test-vectors.csv"

func TestApply_TestVectors(t *testing.T) {
	f, err := os.Open(testVectorsFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"winner_before", "loser_before", "delta", "winner_after", "loser_after", "note"}
	if len(rows) < 2 || !slices.Equal(rows[0], want) {
		t.Fatalf("CSV の見出し = %v, want %v", rows[0], want)
	}
	for i, row := range rows[1:] {
		var v [5]int
		for j := range v {
			if v[j], err = strconv.Atoi(row[j]); err != nil {
				t.Fatalf("%d 行目: %v", i+2, err)
			}
		}
		winner, loser, delta, newWinner, newLoser := v[0], v[1], v[2], v[3], v[4]
		if got := Delta(winner, loser); got != delta {
			t.Errorf("%d 行目: Delta(%d, %d) = %d, want %d", i+2, winner, loser, got, delta)
		}
		w, l := Apply(winner, loser)
		if w != newWinner || l != newLoser {
			t.Errorf("%d 行目: Apply(%d, %d) = (%d, %d), want (%d, %d)", i+2, winner, loser, w, l, newWinner, newLoser)
		}
	}
}

// docs/domain/rating.md の「2.2 早見表」（勝者から見た相手とのレート差 → 変動）。
func TestDelta_QuickReference(t *testing.T) {
	tests := map[int]int{350: 30, 300: 28, 200: 24, 100: 20, 50: 18, 0: 16, -50: 14, -100: 12, -200: 8, -300: 4, -375: 1, -1000: 1, 1000: 30}
	for diff, want := range tests {
		if got := Delta(1500, 1500+diff); got != want {
			t.Errorf("diff %d: Delta = %d, want %d", diff, got, want)
		}
	}
}

func TestFloorDiv(t *testing.T) {
	if got := floorDiv(-489, 25); got != -20 {
		t.Errorf("floorDiv(-489, 25) = %d, want -20", got)
	}
	if got := floorDiv(310, 25); got != 12 {
		t.Errorf("floorDiv(310, 25) = %d, want 12", got)
	}
}

func TestCompute_IndependentOfInputOrder(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a, b, c := shared.AccountID("a"), shared.AccountID("b"), shared.AccountID("c")
	results := []Result{
		{RecordID: "m1", PlayedAt: base, Winner: a, Loser: b},
		{RecordID: "m2", PlayedAt: base.Add(time.Minute), Winner: c, Loser: a},
		{RecordID: "m3", PlayedAt: base.Add(time.Minute), Winner: b, Loser: c}, // m2 と同時刻 → ID 順
		{RecordID: "m4", PlayedAt: base.Add(2 * time.Minute), Winner: a, Loser: c},
	}
	want := Compute(results)
	reversed := []Result{results[3], results[2], results[1], results[0]}
	got := Compute(reversed)
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s: got %+v, want %+v", id, got[id], w)
		}
	}
	// m1: a 1516 / b 1484
	// m2: c 1500 が a 1516 に勝つ → Δ=floor((400+16)/25)=16 → c 1516, a 1500
	// m3: b 1484 が c 1516 に勝つ → Δ=floor((400+32)/25)=17 → b 1501, c 1499
	// m4: a 1500 が c 1499 に勝つ → Δ=floor((400-1)/25)=15 → a 1515, c 1484
	if want[a].Rating != 1515 || want[b].Rating != 1501 || want[c].Rating != 1484 {
		t.Errorf("ratings = a %d, b %d, c %d", want[a].Rating, want[b].Rating, want[c].Rating)
	}
	if want[a].Matches != 3 || want[a].Wins != 2 || want[a].Losses != 1 {
		t.Errorf("a standing = %+v", want[a])
	}
}
