// Package domain は対戦コンテキストのドメインモデルを定義する。
// 進行のルールは docs/domain/match-rules.md に従う。
package domain

import (
	"context"
	"regexp"
	"time"

	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Status は対戦の状態。
type Status string

// 対戦の状態。報告の食い違いは、両者が報告済みで進行中の状態として表す（Mismatched）。
const (
	StatusInProgress Status = "in_progress"
	StatusConfirmed  Status = "confirmed"
	StatusCancelled  Status = "cancelled"
	StatusExpired    Status = "expired"
)

// Report は各プレイヤーの結果報告。
type Report string

// 結果報告の値。
const (
	ReportNone   Report = ""
	ReportWin    Report = "win"
	ReportLose   Report = "lose"
	ReportCancel Report = "cancel"
)

// ExpiryWindow はマッチングから報告が一致するまでの期限。過ぎるとレート変動なしで終了する。
const ExpiryWindow = 24 * time.Hour

var roomIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{5}$`)

// Match は成立した1回の対戦（2本先取）。
type Match struct {
	ID         shared.MatchID
	Player1    shared.AccountID // 部屋を立てる側
	Player2    shared.AccountID // 部屋に入る側
	Fighter1   shared.FighterID // 1戦目のファイター
	Fighter2   shared.FighterID
	RoomID     string
	Status     Status
	Report1    Report
	Report2    Report
	Winner     shared.AccountID
	PlayedAt   time.Time
	ExpiresAt  time.Time
	ResolvedAt time.Time
}

// New は成立した対戦を作る。
func New(id shared.MatchID, p1, p2 shared.AccountID, f1, f2 shared.FighterID, now time.Time) (Match, error) {
	if p1 == p2 {
		return Match{}, shared.Invalid("同じプレイヤー同士は対戦できません")
	}
	return Match{
		ID: id, Player1: p1, Player2: p2, Fighter1: f1, Fighter2: f2,
		Status: StatusInProgress, PlayedAt: now, ExpiresAt: now.Add(ExpiryWindow),
	}, nil
}

// IsParticipant は id が対戦の当事者かを返す。
func (m Match) IsParticipant(id shared.AccountID) bool { return id == m.Player1 || id == m.Player2 }

// Opponent は id の対戦相手を返す。
func (m Match) Opponent(id shared.AccountID) shared.AccountID {
	if id == m.Player1 {
		return m.Player2
	}
	return m.Player1
}

// Mismatched は両者が報告済みで、報告が食い違っているかを返す。
func (m Match) Mismatched() bool {
	return m.Status == StatusInProgress && m.Report1 != ReportNone && m.Report2 != ReportNone
}

// SetRoomID は専用部屋の ID を設定する。設定できるのは部屋を立てる側（プレイヤー1）だけ。
func (m *Match) SetRoomID(by shared.AccountID, roomID string, now time.Time) error {
	if err := m.ensureActive(now); err != nil {
		return err
	}
	if by != m.Player1 {
		return shared.ErrForbidden
	}
	if !roomIDPattern.MatchString(roomID) {
		return shared.Invalid("部屋 ID は半角英数字5文字で入力してください")
	}
	m.RoomID = roomID
	return nil
}

// SubmitReport は by の結果報告を記録し、両者の報告が一致すれば確定・中止にする。
// 確定した場合は confirmed に true を返す。
func (m *Match) SubmitReport(by shared.AccountID, r Report, now time.Time) (confirmed bool, err error) {
	if err := m.ensureActive(now); err != nil {
		return false, err
	}
	if r != ReportWin && r != ReportLose && r != ReportCancel {
		return false, shared.Invalid("報告は「勝ち」「負け」「中止」のいずれかです")
	}
	switch by {
	case m.Player1:
		m.Report1 = r
	case m.Player2:
		m.Report2 = r
	default:
		return false, shared.ErrForbidden
	}
	switch {
	case m.Report1 == ReportWin && m.Report2 == ReportLose:
		m.resolve(StatusConfirmed, m.Player1, now)
	case m.Report1 == ReportLose && m.Report2 == ReportWin:
		m.resolve(StatusConfirmed, m.Player2, now)
	case m.Report1 == ReportCancel && m.Report2 == ReportCancel:
		m.resolve(StatusCancelled, "", now)
	}
	return m.Status == StatusConfirmed, nil
}

// ExpireIfDue は期限を過ぎていれば対戦を期限切れにする。期限切れにした場合は true。
func (m *Match) ExpireIfDue(now time.Time) bool {
	if m.Status != StatusInProgress || now.Before(m.ExpiresAt) {
		return false
	}
	m.resolve(StatusExpired, "", now)
	return true
}

func (m *Match) ensureActive(now time.Time) error {
	if m.ExpireIfDue(now) || m.Status != StatusInProgress {
		return &shared.ValidationError{Message: "この対戦は終了しています"}
	}
	return nil
}

func (m *Match) resolve(s Status, winner shared.AccountID, now time.Time) {
	m.Status = s
	m.Winner = winner
	m.ResolvedAt = now
}

// Repository は対戦の永続化。
type Repository interface {
	Create(ctx context.Context, m Match) error
	Get(ctx context.Context, id shared.MatchID) (Match, error)
	// GetForUpdate は行ロックを取って対戦を取得する（トランザクション内で呼ぶ）。
	GetForUpdate(ctx context.Context, id shared.MatchID) (Match, error)
	Save(ctx context.Context, m Match) error
	// Active は id が当事者の進行中の対戦を返す。なければ ErrNotFound。
	Active(ctx context.Context, id shared.AccountID) (Match, error)
	History(ctx context.Context, id shared.AccountID, limit int) ([]Match, error)
	// ConfirmedBetween は played_at が [from, to) の確定した対戦を返す。to がゼロ値なら上限なし。
	ConfirmedBetween(ctx context.Context, from, to time.Time) ([]Match, error)
	// DueForExpiry は期限を過ぎた進行中の対戦 ID を返す。
	DueForExpiry(ctx context.Context, now time.Time, limit int) ([]shared.MatchID, error)
}
