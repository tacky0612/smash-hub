// Package domain はモデレーションコンテキスト（通報・処分）のドメインモデルを定義する。
// ルールは docs/domain/match-rules.md の「9. 禁止事項とマナー」「10. 通報とモデレーション」に従う。
// 通報と処分はインスタンスのローカルデータで、連合では共有しない（対戦記録の無効化だけが連合で共有される）。
package domain

import (
	"context"
	"slices"
	"time"
	"unicode/utf8"

	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Category は通報・処分の区分（match-rules.md の「9.1 禁止事項」など）。
type Category string

// 通報・処分の区分。
const (
	CategoryCheating      Category = "cheating"       // 改造・チート
	CategoryInputDevice   Category = "input_device"   // 不正な入力機器
	CategoryCollusion     Category = "collusion"      // 八百長
	CategoryFalseReport   Category = "false_report"   // 虚偽の報告
	CategoryUnfairCancel  Category = "unfair_cancel"  // 認められない中止
	CategoryUnauthStream  Category = "unauth_stream"  // 無断配信
	CategoryExposing      Category = "exposing"       // 晒し
	CategoryAbuse         Category = "abuse"          // 暴言・挑発
	CategoryFalseFlagging Category = "false_flagging" // 虚偽の通報（処分の区分としてだけ使う）
	CategoryOther         Category = "other"
)

// ReportCategories は利用者が通報で選べる区分。
var ReportCategories = []Category{
	CategoryCheating, CategoryInputDevice, CategoryCollusion, CategoryFalseReport,
	CategoryUnfairCancel, CategoryUnauthStream, CategoryExposing, CategoryAbuse, CategoryOther,
}

// SanctionCategories は処分の理由として選べる区分。
var SanctionCategories = append(slices.Clone(ReportCategories[:len(ReportCategories)-1]), CategoryFalseFlagging, CategoryOther)

// ReportStatus は通報の対応状況。
type ReportStatus string

// 通報の対応状況。
const (
	ReportOpen     ReportStatus = "open"
	ReportResolved ReportStatus = "resolved"
)

// 通報の制限。
const (
	MaxReportBody   = 1000 // 文字
	MaxNote         = 500  // 文字
	ReportsPerDay   = 5    // 1人が24時間に送れる通報の数
	ReportRetention = 30 * 24 * time.Hour
)

// Report は利用者からの通報。通報したことや通報者は、相手に知らせない。
type Report struct {
	ID         string
	Reporter   shared.AccountID
	Target     shared.AccountID
	MatchHost  shared.InstanceID // 対象の対戦（任意）のホスト
	MatchID    shared.MatchID    // 対象の対戦（任意）
	Category   Category
	Body       string
	Status     ReportStatus
	Resolution string // 運営者の対応メモ（運営者だけが見る）
	CreatedAt  time.Time
	ResolvedAt time.Time
}

// NewReport は入力値を検証して通報を作る。
func NewReport(id string, reporter, target shared.AccountID, matchHost shared.InstanceID, matchID shared.MatchID, category Category, body string, now time.Time) (Report, error) {
	if reporter == target {
		return Report{}, shared.Invalid("自分自身は通報できません")
	}
	if !slices.Contains(ReportCategories, category) {
		return Report{}, shared.Invalid("通報の区分を選んでください")
	}
	if n := utf8.RuneCountInString(body); n < 1 || n > MaxReportBody {
		return Report{}, shared.Invalid("状況を1〜1000文字で書いてください")
	}
	if (matchHost == "") != (matchID == "") {
		return Report{}, shared.Invalid("対象の対戦の指定が正しくありません")
	}
	return Report{
		ID: id, Reporter: reporter, Target: target, MatchHost: matchHost, MatchID: matchID,
		Category: category, Body: body, Status: ReportOpen, CreatedAt: now,
	}, nil
}

// Resolve は通報を対応済みにする。
func (r *Report) Resolve(note string, now time.Time) error {
	if r.Status == ReportResolved {
		return shared.ErrConflict
	}
	if utf8.RuneCountInString(note) > MaxNote {
		return shared.Invalid("対応メモは500文字までです")
	}
	r.Status, r.Resolution, r.ResolvedAt = ReportResolved, note, now
	return nil
}

// SanctionKind は処分の段階（match-rules.md の「10.3 対応の段階」）。
type SanctionKind string

// 処分の段階。
const (
	SanctionCaution SanctionKind = "caution" // 注意。記録には残すが制限はしない
	SanctionYellow  SanctionKind = "yellow"  // イエローカード。警告
	SanctionRed     SanctionKind = "red"     // レッドカード。レーティング対戦の利用停止（期間付きまたは無期限）
)

// Sanction は運営者による処分。モデレーションの記録として残し、削除しない（取り消しは LiftedAt で表す）。
type Sanction struct {
	ID        string
	Account   shared.AccountID
	Kind      SanctionKind
	Reason    Category
	Note      string // 運営者のメモ（運営者だけが見る）
	CreatedAt time.Time
	EndsAt    time.Time // レッドカードの終了日時。ゼロ値なら無期限
	LiftedAt  time.Time // 運営者が解除した日時。ゼロ値なら解除していない
}

// NewSanction は入力値を検証して処分を作る。
func NewSanction(id string, account shared.AccountID, kind SanctionKind, reason Category, note string, endsAt, now time.Time) (Sanction, error) {
	switch kind {
	case SanctionCaution, SanctionYellow:
		if !endsAt.IsZero() {
			return Sanction{}, shared.Invalid("期間を指定できるのはレッドカードだけです")
		}
	case SanctionRed:
		if !endsAt.IsZero() && !endsAt.After(now) {
			return Sanction{}, shared.Invalid("利用停止の終了日時は現在より後にしてください")
		}
	default:
		return Sanction{}, shared.Invalid("処分の段階を選んでください")
	}
	if !slices.Contains(SanctionCategories, reason) {
		return Sanction{}, shared.Invalid("処分の理由を選んでください")
	}
	if utf8.RuneCountInString(note) > MaxNote {
		return Sanction{}, shared.Invalid("メモは500文字までです")
	}
	return Sanction{ID: id, Account: account, Kind: kind, Reason: reason, Note: note, CreatedAt: now, EndsAt: endsAt}, nil
}

// Suspends は now の時点で利用停止中にする処分かを返す。
func (s Sanction) Suspends(now time.Time) bool {
	return s.Kind == SanctionRed && s.LiftedAt.IsZero() && (s.EndsAt.IsZero() || now.Before(s.EndsAt))
}

// Lift は処分を解除する。
func (s *Sanction) Lift(now time.Time) error {
	if !s.LiftedAt.IsZero() {
		return shared.ErrConflict
	}
	s.LiftedAt = now
	return nil
}

// ReportRepository は通報の永続化。
type ReportRepository interface {
	Create(ctx context.Context, r Report) error
	GetForUpdate(ctx context.Context, id string) (Report, error)
	Save(ctx context.Context, r Report) error
	List(ctx context.Context, status ReportStatus, limit int) ([]Report, error)
	CountByReporterSince(ctx context.Context, reporter shared.AccountID, since time.Time) (int, error)
	// PurgeResolved は before より前に対応済みになった通報を削除し、件数を返す
	PurgeResolved(ctx context.Context, before time.Time) (int64, error)
}

// SanctionRepository は処分の永続化。
type SanctionRepository interface {
	Create(ctx context.Context, s Sanction) error
	GetForUpdate(ctx context.Context, id string) (Sanction, error)
	Save(ctx context.Context, s Sanction) error
	ListFor(ctx context.Context, account shared.AccountID) ([]Sanction, error)
}
