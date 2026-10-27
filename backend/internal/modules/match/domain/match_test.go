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
	m, err := New("m1", "https://a.example", "p1", "p2", "mario", "fox", "", t0)
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
			if _, err := m.SubmitReport("p1", Submission{Report: tt.r1}, t0); err != nil {
				t.Fatal(err)
			}
			if m.Status != StatusInProgress {
				t.Fatalf("片方の報告で状態が変わった: %s", m.Status)
			}
			confirmed, err := m.SubmitReport("p2", Submission{Report: tt.r2}, t0)
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
	_, _ = m.SubmitReport("p1", Submission{Report: ReportWin}, t0)
	_, _ = m.SubmitReport("p2", Submission{Report: ReportWin}, t0)
	if !m.Mismatched() {
		t.Fatal("食い違いになっていない")
	}
	confirmed, err := m.SubmitReport("p2", Submission{Report: ReportLose}, t0)
	if err != nil || !confirmed || m.Winner != "p1" {
		t.Fatalf("報告し直しで確定しない: confirmed=%v err=%v winner=%s", confirmed, err, m.Winner)
	}
}

func TestSubmitReport_Rejections(t *testing.T) {
	m := newMatch(t)
	if _, err := m.SubmitReport("outsider", Submission{Report: ReportWin}, t0); !errors.Is(err, shared.ErrForbidden) {
		t.Errorf("当事者以外の報告: err = %v", err)
	}
	if _, err := m.SubmitReport("p1", Submission{Report: "draw"}, t0); err == nil {
		t.Error("不正な報告値を受け付けた")
	}
	if _, err := m.SubmitReport("p1", Submission{Report: ReportWin}, t0.Add(ExpiryWindow)); err == nil {
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

func TestStatementAndRecord(t *testing.T) {
	m := newMatch(t)
	if m.WinnerIndex("p1", ReportWin) != 0 || m.WinnerIndex("p2", ReportLose) != 0 || m.WinnerIndex("p2", ReportWin) != 1 || m.WinnerIndex("p1", ReportCancel) != -1 {
		t.Fatal("WinnerIndex が正しくない")
	}
	// 「p1 の勝ち」と「p2 の負け」は同じ内容に署名することになる
	if string(m.Statement(m.WinnerIndex("p1", ReportWin)).Bytes()) != string(m.Statement(m.WinnerIndex("p2", ReportLose)).Bytes()) {
		t.Fatal("両者の署名対象が一致しない")
	}
	if _, err := m.Record(); err == nil {
		t.Fatal("確定前に記録を作れた")
	}
	_, _ = m.SubmitReport("p1", Submission{Report: ReportWin, Sig: "sig1"}, t0)
	_, _ = m.SubmitReport("p2", Submission{Report: ReportLose, Sig: "sig2"}, t0)
	r, err := m.Record()
	if err != nil {
		t.Fatal(err)
	}
	if r.Winner != 0 || r.Signatures[0].Sig != "sig1" || r.Signatures[1].Sig != "sig2" || r.Picks[0].Characters[1] != "fox" || r.HostInstance != "https://a.example" {
		t.Errorf("記録 = %+v", r)
	}
}

func picks(games ...[2]shared.FighterID) []shared.Pick {
	out := make([]shared.Pick, len(games))
	for i, g := range games {
		out[i] = shared.Pick{Game: i + 1, Characters: []shared.FighterID{g[0], g[1]}}
	}
	return out
}

func TestValidatePicks(t *testing.T) {
	m := newMatch(t) // 1戦目: mario（プレイヤー1） vs fox（プレイヤー2）
	random, err := New("m2", "https://a.example", "p1", "p2", shared.FighterRandom, "fox", "", t0)
	if err != nil {
		t.Fatal(err)
	}
	type g = [2]shared.FighterID
	tests := []struct {
		name   string
		m      Match
		winner int
		picks  []shared.Pick
		ok     bool
	}{
		{"2-0（同じファイター）", m, 0, picks(g{"mario", "fox"}, g{"mario", "fox"}), true},
		{"2-0（1戦目の敗者が変更）", m, 0, picks(g{"mario", "fox"}, g{"mario", "pikachu"}), true},
		{"2-0（1戦目の勝者が変更）", m, 0, picks(g{"mario", "fox"}, g{"link", "fox"}), false},
		{"2-0（プレイヤー2の勝ちで、プレイヤー2が変更）", m, 1, picks(g{"mario", "fox"}, g{"mario", "pikachu"}), false},
		{"2-1（勝者が2戦目に負けた並び）", m, 0, picks(g{"mario", "fox"}, g{"mario", "pikachu"}, g{"link", "pikachu"}), true},
		{"2-1（勝者が1戦目に負けた並び）", m, 0, picks(g{"mario", "fox"}, g{"link", "fox"}, g{"link", "pikachu"}), true},
		{"2-1（勝者が2回続けて変更）", m, 0, picks(g{"mario", "fox"}, g{"link", "fox"}, g{"samus", "fox"}), false},
		{"2-1（敗者が2回続けて変更）", m, 0, picks(g{"mario", "fox"}, g{"mario", "pikachu"}, g{"mario", "ness"}), false},
		{"2-1（変更なし）", m, 0, picks(g{"mario", "fox"}, g{"mario", "fox"}, g{"mario", "fox"}), true},
		{"両者が同時に変更", m, 0, picks(g{"mario", "fox"}, g{"link", "pikachu"}), false},
		{"中止の報告（勝者なし）", m, -1, picks(g{"mario", "fox"}, g{"mario", "fox"}), false},
		{"1ゲームだけ", m, 0, picks(g{"mario", "fox"}), false},
		{"4ゲーム", m, 0, picks(g{"mario", "fox"}, g{"mario", "fox"}, g{"mario", "fox"}, g{"mario", "fox"}), false},
		{"1戦目が登録と違う", m, 0, picks(g{"link", "fox"}, g{"link", "fox"}), false},
		{"random は入れられない", m, 0, picks(g{"mario", "fox"}, g{"mario", shared.FighterRandom}), false},
		{"存在しないファイター", m, 0, picks(g{"mario", "fox"}, g{"mario", "nobody"}), false},
		{"おまかせの側は実際のファイターを入れる", random, 0, picks(g{"link", "fox"}, g{"link", "fox"}), true},
		{"ゲーム番号が連番でない", m, 0, []shared.Pick{{Game: 1, Characters: []shared.FighterID{"mario", "fox"}}, {Game: 3, Characters: []shared.FighterID{"mario", "fox"}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.m.ValidatePicks(tt.winner, tt.picks); (err == nil) != tt.ok {
				t.Errorf("ValidatePicks = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

// 両者のゲームごとのファイターが一致すれば、それを含めた内容とその署名で記録する。
func TestRecord_WithAgreedPicks(t *testing.T) {
	m := newMatch(t)
	p := picks([2]shared.FighterID{"mario", "fox"}, [2]shared.FighterID{"mario", "pikachu"}, [2]shared.FighterID{"link", "pikachu"})
	_, _ = m.SubmitReport("p1", Submission{Report: ReportWin, Sig: "s1", Picks: p, PicksSig: "ps1"}, t0)
	if _, err := m.SubmitReport("p2", Submission{Report: ReportLose, Sig: "s2", Picks: picks([2]shared.FighterID{"mario", "fox"}, [2]shared.FighterID{"mario", "pikachu"}, [2]shared.FighterID{"link", "pikachu"}), PicksSig: "ps2"}, t0); err != nil {
		t.Fatal(err)
	}
	r, err := m.Record()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Picks) != 3 || r.Signatures[0].Sig != "ps1" || r.Signatures[1].Sig != "ps2" {
		t.Errorf("記録 = %+v", r)
	}
	if string(r.MatchStatement.Bytes()) != string(m.StatementWithPicks(0, p).Bytes()) {
		t.Error("署名した内容と記録の内容が違う")
	}
}

// ゲームごとのファイターが食い違う・片方だけ入力した場合も、勝敗が一致すれば確定し、1戦目だけを記録する。
func TestRecord_FallsBackToFirstGame(t *testing.T) {
	cases := map[string][2][]shared.Pick{
		"食い違い":   {picks([2]shared.FighterID{"mario", "fox"}, [2]shared.FighterID{"mario", "fox"}), picks([2]shared.FighterID{"mario", "fox"}, [2]shared.FighterID{"mario", "pikachu"})},
		"片方だけ入力": {picks([2]shared.FighterID{"mario", "fox"}, [2]shared.FighterID{"mario", "fox"}), nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := newMatch(t)
			_, _ = m.SubmitReport("p1", Submission{Report: ReportWin, Sig: "s1", Picks: c[0], PicksSig: "ps1"}, t0)
			confirmed, err := m.SubmitReport("p2", Submission{Report: ReportLose, Sig: "s2", Picks: c[1], PicksSig: "ps2"}, t0)
			if err != nil || !confirmed {
				t.Fatalf("勝敗が一致すれば確定する: confirmed=%v err=%v", confirmed, err)
			}
			r, _ := m.Record()
			if len(r.Picks) != 1 || r.Signatures[0].Sig != "s1" || r.Signatures[1].Sig != "s2" {
				t.Errorf("1戦目だけの内容と署名で記録する: %+v", r)
			}
		})
	}
}

func TestSubmitReport_RejectsInvalidPicks(t *testing.T) {
	m := newMatch(t)
	if _, err := m.SubmitReport("p1", Submission{Report: ReportWin, Picks: picks([2]shared.FighterID{"link", "fox"}, [2]shared.FighterID{"link", "fox"})}, t0); err == nil {
		t.Fatal("ルールに沿わないファイターの報告を受け付けた")
	}
	// 中止の報告ではファイターを記録しない
	if _, err := m.SubmitReport("p1", Submission{Report: ReportCancel, Picks: picks([2]shared.FighterID{"link", "fox"}, [2]shared.FighterID{"link", "fox"})}, t0); err != nil || m.Picks1 != nil {
		t.Fatalf("中止の報告: err=%v picks=%v", err, m.Picks1)
	}
}
