package adapter

import (
	"context"
	"net/http"

	"github.com/tacky0612/smash-hub/backend/internal/modules/rating/application"
	"github.com/tacky0612/smash-hub/backend/internal/platform/httpx"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// DisplayNames は表示名を返す（bootstrap で account を使って実装する）。
type DisplayNames interface {
	DisplayName(ctx context.Context, id shared.AccountID) (string, error)
}

// HTTP は rating の HTTP ハンドラ。
type HTTP struct {
	Service *application.Service
	Names   DisplayNames
}

// Register はルートを登録する。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/ranking", h.ranking)
	mux.HandleFunc("GET /api/accounts/{id}/rating", h.standing)
}

type standingView struct {
	Rank        int              `json:"rank,omitempty"`
	AccountID   shared.AccountID `json:"accountId"`
	DisplayName string           `json:"displayName,omitempty"`
	Rating      int              `json:"rating"`
	Matches     int              `json:"matches"`
	Wins        int              `json:"wins"`
	Losses      int              `json:"losses"`
}

func (h *HTTP) ranking(w http.ResponseWriter, r *http.Request) {
	list, err := h.Service.Ranking(r.Context(), 100)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	views := make([]standingView, 0, len(list))
	for i, s := range list {
		name, err := h.Names.DisplayName(r.Context(), s.AccountID)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		views = append(views, standingView{Rank: i + 1, AccountID: s.AccountID, DisplayName: name,
			Rating: s.Rating, Matches: s.Matches, Wins: s.Wins, Losses: s.Losses})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"season":     h.Service.Season.ID,
		"minMatches": h.Service.MinMatches,
		"entries":    views,
	})
}

func (h *HTTP) standing(w http.ResponseWriter, r *http.Request) {
	id := shared.AccountID(r.PathValue("id"))
	s, err := h.Service.Standing(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	name, err := h.Names.DisplayName(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, standingView{AccountID: id, DisplayName: name,
		Rating: s.Rating, Matches: s.Matches, Wins: s.Wins, Losses: s.Losses})
}
