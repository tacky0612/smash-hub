package domain

import (
	"testing"
	"time"

	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// docs/domain/rating.md の「2.4 テストベクタ」。
func TestApply_TestVectors(t *testing.T) {
	tests := []struct {
		winner, loser, delta, newWinner, newLoser int
	}{
		{1500, 1500, 16, 1516, 1484},
		{1600, 1500, 12, 1612, 1488},
		{1500, 1600, 20, 1520, 1580},
		{1500, 1800, 28, 1528, 1772},
		{1884, 1500, 1, 1885, 1499},
		{2389, 1500, 1, 2390, 1499},
		{1200, 1700, 30, 1230, 1670},
		{1500, 1005, 1, 1501, 1004},
		{1100, 1010, 12, 1112, 1000},
	}
	for _, tt := range tests {
		if got := Delta(tt.winner, tt.loser); got != tt.delta {
			t.Errorf("Delta(%d, %d) = %d, want %d", tt.winner, tt.loser, got, tt.delta)
		}
		w, l := Apply(tt.winner, tt.loser)
		if w != tt.newWinner || l != tt.newLoser {
			t.Errorf("Apply(%d, %d) = (%d, %d), want (%d, %d)", tt.winner, tt.loser, w, l, tt.newWinner, tt.newLoser)
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
