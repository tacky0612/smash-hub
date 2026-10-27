package adapter

import (
	"context"
	"net/http"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/modules/match/application"
	"github.com/tacky0612/smash-hub/backend/internal/modules/match/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/httpx"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// RoomPassword は専用部屋の共通パスワード（docs/domain/match-rules.md の「3. 専用部屋」）。
const RoomPassword = "1122"

// Player は表示用のプレイヤー情報。
type Player struct {
	ID          shared.AccountID `json:"id"`
	DisplayName string           `json:"displayName"`
	Rating      int              `json:"rating"`
}

// Players は表示用のプレイヤー情報を返す（bootstrap で account と rating を組み合わせて実装する）。
type Players interface {
	Player(ctx context.Context, id shared.AccountID) (Player, error)
}

// HTTP は match の HTTP ハンドラ。
type HTTP struct {
	Service *application.Service
	Players Players
}

// Register はルートを登録する。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/matches/active", h.active)
	mux.HandleFunc("GET /api/matches/{id}", h.get)
	mux.HandleFunc("PUT /api/matches/{id}/room", h.setRoom)
	mux.HandleFunc("POST /api/matches/{id}/report", h.report)
	mux.HandleFunc("GET /api/accounts/{id}/matches", h.history)
}

type sideView struct {
	Player   Player           `json:"player"`
	Fighter  shared.FighterID `json:"fighter"`
	Reported bool             `json:"reported"`
	Report   domain.Report    `json:"report,omitempty"` // 両者の報告がそろうまでは相手の報告内容を見せない
	// 自分が報告したゲームごとのファイター（本人にだけ返す）
	Picks []shared.Pick `json:"picks,omitempty"`
}

type matchView struct {
	ID     shared.MatchID    `json:"id"`
	Host   shared.InstanceID `json:"host"`
	Status domain.Status     `json:"status"`
	// Voided はモデレーションで無効化された（レート計算から除外された）確定済みの対戦
	Voided     bool             `json:"voided,omitempty"`
	Mismatched bool             `json:"mismatched"`
	PlayedAt   time.Time        `json:"playedAt"`
	ExpiresAt  time.Time        `json:"expiresAt"`
	Winner     shared.AccountID `json:"winner,omitempty"`
	Player1    sideView         `json:"player1"`
	Player2    sideView         `json:"player2"`
	// 確定した対戦で記録したゲームごとのファイター（両者の入力が一致しなければ1戦目だけ）
	RecordedPicks []shared.Pick `json:"recordedPicks,omitempty"`
	// 当事者にだけ返す
	MyRole       string `json:"myRole,omitempty"` // "player1" / "player2"
	RoomID       string `json:"roomId,omitempty"`
	RoomPassword string `json:"roomPassword,omitempty"`
}

func (h *HTTP) view(ctx context.Context, m domain.Match, viewer shared.AccountID) (matchView, error) {
	p1, err := h.Players.Player(ctx, m.Player1)
	if err != nil {
		return matchView{}, err
	}
	p2, err := h.Players.Player(ctx, m.Player2)
	if err != nil {
		return matchView{}, err
	}
	v := matchView{
		ID: m.ID, Host: m.Host, Status: m.Status, Mismatched: m.Mismatched(), PlayedAt: m.PlayedAt, ExpiresAt: m.ExpiresAt, Winner: m.Winner,
		Player1: sideView{Player: p1, Fighter: m.Fighter1, Reported: m.Report1 != domain.ReportNone},
		Player2: sideView{Player: p2, Fighter: m.Fighter2, Reported: m.Report2 != domain.ReportNone},
	}
	if m.Status == domain.StatusConfirmed {
		v.RecordedPicks = m.RecordedPicks()
	}
	if !m.IsParticipant(viewer) {
		return v, nil
	}
	v.RoomID, v.RoomPassword = m.RoomID, RoomPassword
	bothReported := v.Player1.Reported && v.Player2.Reported
	if viewer == m.Player1 {
		v.MyRole = "player1"
		v.Player1.Report, v.Player1.Picks = m.Report1, m.Picks1
		if bothReported {
			v.Player2.Report = m.Report2
		}
	} else {
		v.MyRole = "player2"
		v.Player2.Report, v.Player2.Picks = m.Report2, m.Picks2
		if bothReported {
			v.Player1.Report = m.Report1
		}
	}
	return v, nil
}

func (h *HTTP) write(w http.ResponseWriter, r *http.Request, m domain.Match, viewer shared.AccountID) {
	v, err := h.view(r.Context(), m, viewer)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	voided, err := h.Service.Voided(r.Context(), m)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	v.Voided = voided[application.MatchRef{Host: m.Host, ID: m.ID}]
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (h *HTTP) active(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	id, err := h.Service.ActiveMatchID(r.Context(), me)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"matchId": id})
}

func (h *HTTP) get(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	m, err := h.Service.Get(r.Context(), shared.MatchID(r.PathValue("id")))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.write(w, r, m, me)
}

func (h *HTTP) setRoom(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req struct {
		RoomID string `json:"roomId"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	m, err := h.Service.SetRoomID(r.Context(), me, shared.MatchID(r.PathValue("id")), req.RoomID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.write(w, r, m, me)
}

func (h *HTTP) report(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req struct {
		Result domain.Report `json:"result"`
		// ゲームごとのファイター（任意）。プレイヤー1・プレイヤー2の順の2人分を、1戦目から順に
		Picks []shared.Pick `json:"picks"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if len(req.Picks) == 0 {
		req.Picks = nil
	}
	m, err := h.Service.Report(r.Context(), me, shared.MatchID(r.PathValue("id")), req.Result, req.Picks)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.write(w, r, m, me)
}

func (h *HTTP) history(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	ms, err := h.Service.History(r.Context(), shared.AccountID(r.PathValue("id")), 20)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	voided, err := h.Service.Voided(r.Context(), ms...)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	views := make([]matchView, 0, len(ms))
	for _, m := range ms {
		v, err := h.view(r.Context(), m, me)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		v.Voided = voided[application.MatchRef{Host: m.Host, ID: m.ID}]
		v.RoomID, v.RoomPassword = "", "" // 一覧には部屋情報を含めない
		views = append(views, v)
	}
	httpx.WriteJSON(w, http.StatusOK, views)
}
