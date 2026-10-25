package domain

import (
	"testing"
	"time"

	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

var t0 = time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC)

func req(t *testing.T, id shared.AccountID, rateRange, rating int, at time.Time) Request {
	t.Helper()
	r, err := NewRequest(id, "mario", rateRange, rating, at)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestNewRequest_Validation(t *testing.T) {
	if _, err := NewRequest("a", "unknown", 0, 1500, t0); err == nil {
		t.Error("存在しないファイターを受け付けた")
	}
	if _, err := NewRequest("a", "mario", 75, 1500, t0); err == nil {
		t.Error("選択肢にないレート差を受け付けた")
	}
	if _, err := NewRequest("a", shared.FighterRandom, 0, 1500, t0); err != nil {
		t.Errorf("おまかせを受け付けない: %v", err)
	}
}

func TestAllowedDiff_UnlimitedExpandsEveryMinute(t *testing.T) {
	r := req(t, "a", 0, 1500, t0)
	for _, tt := range []struct {
		after time.Duration
		want  int
	}{{0, 200}, {59 * time.Second, 200}, {time.Minute, 300}, {3*time.Minute + 30*time.Second, 500}} {
		if got := r.AllowedDiff(t0.Add(tt.after)); got != tt.want {
			t.Errorf("%v 後: %d, want %d", tt.after, got, tt.want)
		}
	}
	if got := req(t, "b", 100, 1500, t0).AllowedDiff(t0.Add(time.Hour)); got != 100 {
		t.Errorf("指定ありのレート差が広がった: %d", got)
	}
}

func TestCompatible(t *testing.T) {
	a := req(t, "a", 0, 1500, t0)
	if !Compatible(a, req(t, "b", 0, 1700, t0), t0) {
		t.Error("制限なし同士でレート差200がマッチしない")
	}
	if Compatible(a, req(t, "b", 0, 1701, t0), t0) {
		t.Error("制限なし同士でレート差201がマッチした")
	}
	if Compatible(a, req(t, "b", 50, 1551, t0), t0) {
		t.Error("狭いほうのレート差（±50）が使われていない")
	}
	if !Compatible(a, req(t, "b", 0, 1800, t0), t0.Add(time.Minute)) {
		t.Error("1分待つとレート差300までマッチするはず")
	}
	if Compatible(a, req(t, "a", 0, 1500, t0), t0) {
		t.Error("自分自身とマッチした")
	}
}

func TestPair_EarlierRequestIsPlayer1(t *testing.T) {
	early := req(t, "a", 0, 1500, t0)
	late := req(t, "b", 0, 1500, t0.Add(time.Second))
	if p1, p2 := Pair(late, early); p1.AccountID != "a" || p2.AccountID != "b" {
		t.Errorf("p1=%s p2=%s", p1.AccountID, p2.AccountID)
	}
}

func TestIsStale(t *testing.T) {
	r := req(t, "a", 0, 1500, t0)
	if r.IsStale(t0.Add(StaleAfter)) || !r.IsStale(t0.Add(StaleAfter+time.Second)) {
		t.Error("離脱の判定が正しくない")
	}
}
