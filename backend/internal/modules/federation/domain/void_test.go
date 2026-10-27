package domain

import (
	"strings"
	"testing"
	"time"
)

func voidEvent(t *testing.T, payload any) Event {
	t.Helper()
	e, err := NewEvent(TypeMatchVoided, host, 2, "prev", "2026-11-02T20:00:00.000Z", payload, newKey().sign)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestDecodeMatchVoided(t *testing.T) {
	id := strings.Repeat("ab", 32)
	v, err := DecodeMatchVoided(voidEvent(t, MatchVoided{RecordID: id, Reason: VoidCollusion}))
	if err != nil || v.RecordID != id || v.Reason != VoidCollusion {
		t.Fatalf("v=%+v err=%v", v, err)
	}
	// 新しいバージョンで追加された事由も、書式が正しければ取り込む
	if _, err := DecodeMatchVoided(voidEvent(t, MatchVoided{RecordID: id, Reason: "new_reason"})); err != nil {
		t.Errorf("未知の事由: %v", err)
	}
	for name, p := range map[string]MatchVoided{
		"記録 ID が短い":  {RecordID: "abc", Reason: VoidOther},
		"記録 ID が大文字": {RecordID: strings.Repeat("AB", 32), Reason: VoidOther},
		"事由が空":       {RecordID: id},
		"事由に自由記述":    {RecordID: id, Reason: "対戦相手が八百長"},
	} {
		if _, err := DecodeMatchVoided(voidEvent(t, p)); err == nil {
			t.Errorf("%s: 受理してしまう", name)
		}
	}
}

// 無効化は、played_at からホストがログに追記するまでが 7日以内のときだけ有効（確定レートを変えない）。
func TestVoidEffective(t *testing.T) {
	played := time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC)
	cases := []struct {
		after time.Duration
		want  bool
	}{
		{time.Hour, true},
		{VoidDeadline, true},
		{VoidDeadline + time.Millisecond, false},
	}
	for _, c := range cases {
		if got := VoidEffective(played, played.Add(c.after)); got != c.want {
			t.Errorf("%v 後: got %v, want %v", c.after, got, c.want)
		}
	}
}
