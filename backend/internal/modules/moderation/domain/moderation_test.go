package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

var now = time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC)

func TestNewReport(t *testing.T) {
	if _, err := NewReport("r1", "did:key:a", "did:key:b", "https://a.example", "m1", CategoryCollusion, "わざと負けていた", now); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func() (Report, error){
		"自分自身": func() (Report, error) {
			return NewReport("r1", "did:key:a", "did:key:a", "", "", CategoryAbuse, "x", now)
		},
		"区分がない": func() (Report, error) {
			return NewReport("r1", "did:key:a", "did:key:b", "", "", "", "x", now)
		},
		"処分専用の区分": func() (Report, error) {
			return NewReport("r1", "did:key:a", "did:key:b", "", "", CategoryFalseFlagging, "x", now)
		},
		"本文が空": func() (Report, error) {
			return NewReport("r1", "did:key:a", "did:key:b", "", "", CategoryAbuse, "", now)
		},
		"本文が長すぎる": func() (Report, error) {
			return NewReport("r1", "did:key:a", "did:key:b", "", "", CategoryAbuse, strings.Repeat("あ", MaxReportBody+1), now)
		},
		"対戦のホストだけ": func() (Report, error) {
			return NewReport("r1", "did:key:a", "did:key:b", "https://a.example", "", CategoryAbuse, "x", now)
		},
	}
	for name, f := range cases {
		var ve *shared.ValidationError
		if _, err := f(); !errors.As(err, &ve) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestReport_ResolveOnce(t *testing.T) {
	r, _ := NewReport("r1", "did:key:a", "did:key:b", "", "", CategoryAbuse, "x", now)
	if err := r.Resolve("注意した", now.Add(time.Hour)); err != nil || r.Status != ReportResolved {
		t.Fatalf("r=%+v err=%v", r, err)
	}
	if err := r.Resolve("", now); !errors.Is(err, shared.ErrConflict) {
		t.Errorf("2回目の対応: %v", err)
	}
}

func TestSanction_Suspends(t *testing.T) {
	week := now.Add(7 * 24 * time.Hour)
	red, err := NewSanction("s1", "did:key:b", SanctionRed, CategoryCollusion, "", week, now)
	if err != nil {
		t.Fatal(err)
	}
	if !red.Suspends(now) || red.Suspends(week) {
		t.Errorf("期間付きの利用停止: 開始時 %v、終了時 %v", red.Suspends(now), red.Suspends(week))
	}
	forever, _ := NewSanction("s2", "did:key:b", SanctionRed, CategoryCheating, "", time.Time{}, now)
	if !forever.Suspends(now.Add(10 * 365 * 24 * time.Hour)) {
		t.Error("無期限の利用停止が終わる")
	}
	if err := forever.Lift(now); err != nil || forever.Suspends(now) {
		t.Errorf("解除後も利用停止: %v", err)
	}
	if err := forever.Lift(now); !errors.Is(err, shared.ErrConflict) {
		t.Errorf("2回目の解除: %v", err)
	}
	yellow, _ := NewSanction("s3", "did:key:b", SanctionYellow, CategoryUnfairCancel, "", time.Time{}, now)
	if yellow.Suspends(now) {
		t.Error("イエローカードで利用停止になる")
	}
}

func TestNewSanction_Validation(t *testing.T) {
	cases := map[string]func() (Sanction, error){
		"注意に期間": func() (Sanction, error) {
			return NewSanction("s", "a", SanctionCaution, CategoryAbuse, "", now.Add(time.Hour), now)
		},
		"終了が過去": func() (Sanction, error) {
			return NewSanction("s", "a", SanctionRed, CategoryAbuse, "", now, now)
		},
		"段階がない": func() (Sanction, error) {
			return NewSanction("s", "a", "", CategoryAbuse, "", time.Time{}, now)
		},
		"理由がない": func() (Sanction, error) {
			return NewSanction("s", "a", SanctionYellow, "", "", time.Time{}, now)
		},
	}
	for name, f := range cases {
		var ve *shared.ValidationError
		if _, err := f(); !errors.As(err, &ve) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	// 虚偽の通報は処分の理由にできる
	if _, err := NewSanction("s", "a", SanctionYellow, CategoryFalseFlagging, "", time.Time{}, now); err != nil {
		t.Error(err)
	}
}
