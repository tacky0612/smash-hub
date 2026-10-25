package adapter

import (
	"net/http"

	"github.com/tacky0612/smash-hub/backend/internal/modules/matchmaking/application"
	"github.com/tacky0612/smash-hub/backend/internal/platform/httpx"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// HTTP は matchmaking の HTTP ハンドラ。
type HTTP struct {
	Service *application.Service
}

// Register はルートを登録する。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/matchmaking/search", h.search)
	mux.HandleFunc("GET /api/matchmaking/status", h.status)
	mux.HandleFunc("DELETE /api/matchmaking/search", h.cancel)
}

type statusView struct {
	State          application.State `json:"state"`
	MatchID        shared.MatchID    `json:"matchId,omitempty"`
	WaitingSeconds int               `json:"waitingSeconds,omitempty"`
	AllowedDiff    int               `json:"allowedDiff,omitempty"`
}

func toView(s application.Status) statusView {
	return statusView{State: s.State, MatchID: s.MatchID, WaitingSeconds: int(s.WaitingFor.Seconds()), AllowedDiff: s.AllowedDiff}
}

func (h *HTTP) search(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req struct {
		Fighter   shared.FighterID `json:"fighter"`
		RateRange int              `json:"rateRange"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	st, err := h.Service.Search(r.Context(), id, req.Fighter, req.RateRange)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toView(st))
}

func (h *HTTP) status(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	st, err := h.Service.Status(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toView(st))
}

func (h *HTTP) cancel(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.Service.Cancel(r.Context(), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
