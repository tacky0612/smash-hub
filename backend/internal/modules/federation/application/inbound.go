package application

import (
	"context"
	"time"

	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// 連合先から届くインスタンスをまたぐ対戦の要求を、matchmaking・match コンテキストに渡すための port。
// bootstrap で各コンテキストのユースケースに接続する。

// Claim はホストへの対戦の申し込み（配信された募集に対して、探している側のインスタンスが送る）。
type Claim struct {
	RequestID string           `json:"request_id"`
	AccountID shared.AccountID `json:"account_id"`
	Fighter   shared.FighterID `json:"fighter"`
	RateRange int              `json:"rate_range"`
	CreatedAt time.Time        `json:"created_at"`
}

// LiveMatch はホストが参加者のホームインスタンスに返す、進行中の対戦の状態。
type LiveMatch struct {
	MatchID     shared.MatchID   `json:"match_id"`
	Host        string           `json:"host"`
	Player1     shared.AccountID `json:"player1"`
	Player2     shared.AccountID `json:"player2"`
	Player2Home string           `json:"player2_home"`
	Fighter1    shared.FighterID `json:"fighter1"`
	Fighter2    shared.FighterID `json:"fighter2"`
	RoomID      string           `json:"room_id"`
	Status      string           `json:"status"`
	Report1     string           `json:"report1"`
	Report2     string           `json:"report2"`
	Winner      shared.AccountID `json:"winner"`
	PlayedAt    time.Time        `json:"played_at"`
	ExpiresAt   time.Time        `json:"expires_at"`
}

// RemoteReport は参加者のホームインスタンスからホストへ中継される結果報告。
type RemoteReport struct {
	MatchID   shared.MatchID   `json:"match_id"`
	AccountID shared.AccountID `json:"account_id"`
	Result    string           `json:"result"`
	Sig       string           `json:"sig"` // 報告に対応する MatchStatement へのプレイヤーの署名（中止の場合は空）
}

// InboundMatchmaking はホストとして対戦の申し込みを受け付ける。
type InboundMatchmaking interface {
	AcceptClaim(ctx context.Context, from string, c Claim) (shared.MatchID, error)
}

// InboundMatches はホストとして対戦の状態の提供と、中継された報告を受け付ける。
type InboundMatches interface {
	Live(ctx context.Context, from string, id shared.MatchID) (LiveMatch, error)
	Report(ctx context.Context, from string, r RemoteReport) (LiveMatch, error)
}
