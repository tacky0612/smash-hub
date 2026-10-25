package domain

import (
	"errors"
	"testing"
	"time"

	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

var t0 = time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC)

func newMatch(t *testing.T) Match {
	t.Helper()
	m, err := New("m1", "p1", "p2", "mario", "fox", t0)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSubmitReport(t *testing.T) {
	tests := []struct {
		name       string
		r1, r2     Report
		wantStatus Status
		wantWinner shared.AccountID
		mismatched bool
	}{
		{"プレイヤー1の勝ち", ReportWin, ReportLose, StatusConfirmed, "p1", false},
		{"プレイヤー2の勝ち", ReportLose, ReportWin, StatusConfirmed, "p2", false},
		{"両者中止", ReportCancel, ReportCancel, StatusCancelled, "", false},
		{"勝ち-勝ちは食い違い", ReportWin, ReportWin, StatusInProgress, "", true},
		{"負け-負けは食い違い", ReportLose, ReportLose, StatusInProgress, "", true},
		{"勝ち-中止は食い違い", ReportWin, ReportCancel, StatusInProgress, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMatch(t)
			if _, err := m.SubmitReport("p1", tt.r1, t0); err != nil {
				t.Fatal(err)
			}
			if m.Status != StatusInProgress {
				t.Fatalf("片方の報告で状態が変わった: %s", m.Status)
			}
			confirmed, err := m.SubmitReport("p2", tt.r2, t0)
			if err != nil {
				t.Fatal(err)
			}
			if m.Status != tt.wantStatus || m.Winner != tt.wantWinner || m.Mismatched() != tt.mismatched {
				t.Errorf("status=%s winner=%s mismatched=%v", m.Status, m.Winner, m.Mismatched())
			}
			if confirmed != (tt.wantStatus == StatusConfirmed) {
				t.Errorf("confirmed = %v", confirmed)
			}
		})
	}
}

func TestSubmitReport_ReReportResolvesMismatch(t *testing.T) {
	m := newMatch(t)
	_, _ = m.SubmitReport("p1", ReportWin, t0)
	_, _ = m.SubmitReport("p2", ReportWin, t0)
	if !m.Mismatched() {
		t.Fatal("食い違いになっていない")
	}
	confirmed, err := m.SubmitReport("p2", ReportLose, t0)
	if err != nil || !confirmed || m.Winner != "p1" {
		t.Fatalf("報告し直しで確定しない: confirmed=%v err=%v winner=%s", confirmed, err, m.Winner)
	}
}

func TestSubmitReport_Rejections(t *testing.T) {
	m := newMatch(t)
	if _, err := m.SubmitReport("outsider", ReportWin, t0); !errors.Is(err, shared.ErrForbidden) {
		t.Errorf("当事者以外の報告: err = %v", err)
	}
	if _, err := m.SubmitReport("p1", "draw", t0); err == nil {
		t.Error("不正な報告値を受け付けた")
	}
	if _, err := m.SubmitReport("p1", ReportWin, t0.Add(ExpiryWindow)); err == nil {
		t.Error("期限後の報告を受け付けた")
	}
	if m.Status != StatusExpired {
		t.Errorf("期限後の状態 = %s", m.Status)
	}
}

func TestSetRoomID(t *testing.T) {
	m := newMatch(t)
	if err := m.SetRoomID("p2", "AB12C", t0); !errors.Is(err, shared.ErrForbidden) {
		t.Errorf("プレイヤー2が設定できた: %v", err)
	}
	if err := m.SetRoomID("p1", "AB-12", t0); err == nil {
		t.Error("不正な部屋 ID を受け付けた")
	}
	if err := m.SetRoomID("p1", "AB12C", t0); err != nil || m.RoomID != "AB12C" {
		t.Errorf("部屋 ID を設定できない: %v", err)
	}
}

func TestExpireIfDue(t *testing.T) {
	m := newMatch(t)
	if m.ExpireIfDue(t0.Add(ExpiryWindow - time.Second)) {
		t.Error("期限前に期限切れになった")
	}
	if !m.ExpireIfDue(t0.Add(ExpiryWindow)) || m.Status != StatusExpired {
		t.Error("期限で期限切れにならない")
	}
}
