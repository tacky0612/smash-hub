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
	// Host はこの対戦をホストするインスタンス（結果の受付と確定を担当する）
	Host shared.InstanceID
	// Player2Home はプレイヤー2が他インスタンスの利用者の場合のホームインスタンス（自インスタンスの利用者なら空）
	Player2Home shared.InstanceID
	// Sig1・Sig2 は各プレイヤーの結果報告の署名（報告が示す勝者の、1戦目だけの MatchStatement への署名）
	Sig1 string
	Sig2 string
	// Picks1・Picks2 は各プレイヤーが報告したゲームごとのファイター（任意。入力しなければ nil）。
	// PicksSig1・PicksSig2 は、そのファイターを含めた MatchStatement への署名。
	// 両者の入力が一致したときだけ、ゲームごとのファイターを記録に含める（docs/adr/0010-game-picks-in-report.md）
	Picks1    []shared.Pick
	Picks2    []shared.Pick
	PicksSig1 string
	PicksSig2 string
}

// Submission は1人のプレイヤーの結果報告。
type Submission struct {
	Report Report
	Sig    string        // 1戦目だけの対戦結果の内容への署名（勝敗の報告のとき）
	Picks  []shared.Pick // ゲームごとのファイター（任意）
	// PicksSig は Picks を含めた対戦結果の内容への署名（Picks を入力したとき）
	PicksSig string
}

// New は成立した対戦を作る。p2Home はプレイヤー2が他インスタンスの利用者ならそのインスタンス。
func New(id shared.MatchID, host shared.InstanceID, p1, p2 shared.AccountID, f1, f2 shared.FighterID, p2Home shared.InstanceID, now time.Time) (Match, error) {
	if p1 == p2 {
		return Match{}, shared.Invalid("同じプレイヤー同士は対戦できません")
	}
	return Match{
		ID: id, Host: host, Player1: p1, Player2: p2, Fighter1: f1, Fighter2: f2, Player2Home: p2Home,
		Status: StatusInProgress, PlayedAt: now, ExpiresAt: now.Add(ExpiryWindow),
	}, nil
}

// WinnerIndex は by の報告 r が示す勝者の添字（0: プレイヤー1、1: プレイヤー2）を返す。勝敗の報告でなければ -1。
func (m Match) WinnerIndex(by shared.AccountID, r Report) int {
	self := 0
	if by == m.Player2 {
		self = 1
	}
	switch r {
	case ReportWin:
		return self
	case ReportLose:
		return 1 - self
	}
	return -1
}

// Statement は勝者を winner とした、1戦目のファイターだけの対戦結果の内容（両プレイヤーが署名する対象）を返す。
// 1戦目のファイターはマッチングで登録したもの（おまかせなら random）。
func (m Match) Statement(winner int) shared.MatchStatement {
	return m.StatementWithPicks(winner, nil)
}

// StatementWithPicks は、ゲームごとのファイター picks を含めた対戦結果の内容を返す。picks が nil なら Statement と同じ。
func (m Match) StatementWithPicks(winner int, picks []shared.Pick) shared.MatchStatement {
	if picks == nil {
		picks = []shared.Pick{{Game: 1, Characters: []shared.FighterID{m.Fighter1, m.Fighter2}}}
	}
	return shared.MatchStatement{
		MatchID:      m.ID,
		HostInstance: m.Host,
		PlayedAt:     shared.FormatTime(m.PlayedAt),
		Ruleset:      shared.RulesetStandard,
		Players:      []shared.PlayerRef{{AccountID: m.Player1}, {AccountID: m.Player2}},
		Picks:        picks,
		Winner:       winner,
	}
}

// Record は確定した対戦の記録（両者の署名付き）を返す。
func (m Match) Record() (shared.MatchRecord, error) {
	if m.Status != StatusConfirmed {
		return shared.MatchRecord{}, shared.Invalid("確定していない対戦です")
	}
	winner := 0
	if m.Winner == m.Player2 {
		winner = 1
	}
	if m.PicksAgreed() {
		return shared.MatchRecord{
			MatchStatement: m.StatementWithPicks(winner, m.Picks1),
			Signatures: []shared.PlayerSignature{
				{AccountID: m.Player1, Sig: m.PicksSig1},
				{AccountID: m.Player2, Sig: m.PicksSig2},
			},
		}, nil
	}
	return shared.MatchRecord{
		MatchStatement: m.Statement(winner),
		Signatures: []shared.PlayerSignature{
			{AccountID: m.Player1, Sig: m.Sig1},
			{AccountID: m.Player2, Sig: m.Sig2},
		},
	}, nil
}

// PicksAgreed は、両者がゲームごとのファイターを報告し、その内容が一致しているかを返す。
func (m Match) PicksAgreed() bool {
	return m.Picks1 != nil && m.Picks2 != nil && shared.SamePicks(m.Picks1, m.Picks2)
}

// RecordedPicks は確定した対戦の記録に含めるゲームごとのファイターを返す。
func (m Match) RecordedPicks() []shared.Pick {
	if m.PicksAgreed() {
		return m.Picks1
	}
	return m.Statement(0).Picks
}

// ValidatePicks は、勝者を winner（Players の添字）とした報告のゲームごとのファイター picks が標準ルールに沿っているかを確認する。
//   - 2〜3ゲーム（2本先取）で、game は1からの連番、characters はプレイヤー1・プレイヤー2の順の2人分
//   - 実際に使ったファイターを入れる（random は不可）
//   - 1戦目はマッチングで登録したファイター（おまかせで登録した側は、実際に使ったファイター）
//   - 2・3戦目にファイターを変えられるのは直前のゲームの敗者だけ。ゲームごとの勝敗は報告しないが、
//     対戦の勝者から各ゲームの勝敗の並びは限られるため、それと矛盾しないことを確かめる
//     （2ゲームなら勝者の2連勝、3ゲームなら勝者が1戦目か2戦目のどちらかで負けている）
func (m Match) ValidatePicks(winner int, picks []shared.Pick) error {
	if len(picks) < 2 || len(picks) > shared.MaxGames {
		return shared.Invalid("ゲームごとのファイターは2〜3ゲーム分を入力してください")
	}
	if winner != 0 && winner != 1 {
		return shared.Invalid("ゲームごとのファイターは勝敗の報告と一緒に入力してください")
	}
	registered := []shared.FighterID{m.Fighter1, m.Fighter2}
	for i, p := range picks {
		if p.Game != i+1 || len(p.Characters) != 2 {
			return shared.Invalid("ゲームごとのファイターの形式が正しくありません")
		}
		for j, c := range p.Characters {
			if !shared.IsValidFighter(c, false) {
				return shared.Invalid("ゲームごとのファイターには実際に使ったファイターを選んでください")
			}
			if i == 0 && registered[j] != shared.FighterRandom && c != registered[j] {
				return shared.Invalid("1戦目のファイターはマッチングで選んだファイターです")
			}
		}
	}
	// candidates は各ゲームの勝者の並びの候補（最後のゲームは対戦の勝者が勝つ）
	loser := 1 - winner
	candidates := [][]int{{winner, winner}}
	if len(picks) == 3 {
		candidates = [][]int{{loser, winner, winner}, {winner, loser, winner}}
	}
	for _, gameWinners := range candidates {
		if onlyLosersChanged(picks, gameWinners) {
			return nil
		}
	}
	return shared.Invalid("ファイターを変えられるのは直前のゲームの敗者だけです")
}

// onlyLosersChanged は、各ゲームの勝者が gameWinners のとき、続くゲームでファイターを変えたのが直前の敗者だけかを返す。
func onlyLosersChanged(picks []shared.Pick, gameWinners []int) bool {
	for i := 1; i < len(picks); i++ {
		prevWinner := gameWinners[i-1]
		if picks[i].Characters[prevWinner] != picks[i-1].Characters[prevWinner] {
			return false
		}
	}
	return true
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

// SubmitReport は by の結果報告（勝敗・署名・任意でゲームごとのファイター）を記録し、
// 両者の報告が一致すれば確定・中止にする。確定した場合は confirmed に true を返す。
// 確定の条件は勝敗の一致だけで、ゲームごとのファイターが食い違っても確定する（その場合は1戦目だけを記録する）。
func (m *Match) SubmitReport(by shared.AccountID, s Submission, now time.Time) (confirmed bool, err error) {
	if err := m.ensureActive(now); err != nil {
		return false, err
	}
	if s.Report != ReportWin && s.Report != ReportLose && s.Report != ReportCancel {
		return false, shared.Invalid("報告は「勝ち」「負け」「中止」のいずれかです")
	}
	if s.Report == ReportCancel {
		s.Picks, s.PicksSig = nil, "" // 中止した対戦のファイターは記録しない
	}
	if s.Picks != nil {
		if err := m.ValidatePicks(m.WinnerIndex(by, s.Report), s.Picks); err != nil {
			return false, err
		}
	}
	switch by {
	case m.Player1:
		m.Report1, m.Sig1, m.Picks1, m.PicksSig1 = s.Report, s.Sig, s.Picks, s.PicksSig
	case m.Player2:
		m.Report2, m.Sig2, m.Picks2, m.PicksSig2 = s.Report, s.Sig, s.Picks, s.PicksSig
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

// RemoteRef は、自インスタンスの利用者が参加している、他インスタンスがホストする対戦への参照。
// 対戦の状態はホストが持ち、自インスタンスは参照と最後に確認した状態だけを持つ。
type RemoteRef struct {
	MatchID   shared.MatchID
	Host      shared.InstanceID
	AccountID shared.AccountID
	Status    Status
	CreatedAt time.Time
}

// RemoteRefRepository は RemoteRef の永続化。
type RemoteRefRepository interface {
	Create(ctx context.Context, r RemoteRef) error
	Get(ctx context.Context, id shared.MatchID) (RemoteRef, error)
	Active(ctx context.Context, account shared.AccountID) ([]RemoteRef, error)
	SetStatus(ctx context.Context, id shared.MatchID, s Status) error
}
