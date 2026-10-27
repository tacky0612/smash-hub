package adapter

import (
	"context"
	"net/http"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/modules/moderation/application"
	"github.com/tacky0612/smash-hub/backend/internal/modules/moderation/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/httpx"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// DisplayNames は表示名を返す（bootstrap で account と federation を使って実装する）。
type DisplayNames interface {
	DisplayName(ctx context.Context, id shared.AccountID) (string, error)
}

// HTTP は moderation の HTTP ハンドラ。
type HTTP struct {
	Service *application.Service
	Names   DisplayNames
	Self    shared.InstanceID
	Clock   shared.Clock
}

// Register はルートを登録する。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/reports", h.report)
	mux.HandleFunc("GET /api/me/moderation", h.status)
	mux.HandleFunc("GET /api/admin/reports", h.listReports)
	mux.HandleFunc("POST /api/admin/reports/{id}/resolve", h.resolveReport)
	mux.HandleFunc("GET /api/admin/accounts/{id}/sanctions", h.sanctionsFor)
	mux.HandleFunc("POST /api/admin/sanctions", h.sanction)
	mux.HandleFunc("POST /api/admin/sanctions/{id}/lift", h.lift)
	mux.HandleFunc("POST /api/admin/voids", h.void)
}

type accountRef struct {
	ID          shared.AccountID `json:"id"`
	DisplayName string           `json:"displayName"`
}

func (h *HTTP) ref(ctx context.Context, id shared.AccountID) (accountRef, error) {
	name, err := h.Names.DisplayName(ctx, id)
	return accountRef{ID: id, DisplayName: name}, err
}

// sanctionView は処分。note は運営者にだけ返す。
type sanctionView struct {
	ID        string              `json:"id"`
	Kind      domain.SanctionKind `json:"kind"`
	Reason    domain.Category     `json:"reason"`
	Note      string              `json:"note,omitempty"`
	CreatedAt time.Time           `json:"createdAt"`
	EndsAt    *time.Time          `json:"endsAt,omitempty"`
	LiftedAt  *time.Time          `json:"liftedAt,omitempty"`
	Active    bool                `json:"active"` // 利用停止中
}

func (h *HTTP) sanctionView(s domain.Sanction, withNote bool) sanctionView {
	v := sanctionView{ID: s.ID, Kind: s.Kind, Reason: s.Reason, CreatedAt: s.CreatedAt, EndsAt: nullTime(s.EndsAt), LiftedAt: nullTime(s.LiftedAt), Active: s.Suspends(h.Clock.Now())}
	if withNote {
		v.Note = s.Note
	}
	return v
}

func (h *HTTP) sanctionViews(list []domain.Sanction, withNote bool) []sanctionView {
	out := make([]sanctionView, 0, len(list))
	for _, s := range list {
		out = append(out, h.sanctionView(s, withNote))
	}
	return out
}

type reportView struct {
	ID         string              `json:"id"`
	Reporter   accountRef          `json:"reporter"`
	Target     accountRef          `json:"target"`
	MatchHost  shared.InstanceID   `json:"matchHost,omitempty"`
	MatchID    shared.MatchID      `json:"matchId,omitempty"`
	HostedHere bool                `json:"hostedHere"` // 対象の対戦を自インスタンスがホストした（無効化できる）
	Category   domain.Category     `json:"category"`
	Body       string              `json:"body"`
	Status     domain.ReportStatus `json:"status"`
	Resolution string              `json:"resolution,omitempty"`
	CreatedAt  time.Time           `json:"createdAt"`
	ResolvedAt *time.Time          `json:"resolvedAt,omitempty"`
}

func (h *HTTP) report(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req struct {
		TargetID  shared.AccountID  `json:"targetId"`
		MatchHost shared.InstanceID `json:"matchHost"`
		MatchID   shared.MatchID    `json:"matchId"`
		Category  domain.Category   `json:"category"`
		Body      string            `json:"body"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if _, err := h.Service.Report(r.Context(), me, req.TargetID, req.MatchHost, req.MatchID, req.Category, req.Body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	// 通報の内容は通報者にも返さない（運営者だけが見る）
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTP) status(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	st, err := h.Service.StatusOf(r.Context(), me)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	res := map[string]any{"admin": st.Admin, "sanctions": h.sanctionViews(st.Sanctions, false)}
	if st.Suspension != nil {
		res["suspension"] = h.sanctionView(*st.Suspension, false)
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *HTTP) listReports(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	status := domain.ReportStatus(r.URL.Query().Get("status"))
	if status == "" {
		status = domain.ReportOpen
	}
	list, err := h.Service.ListReports(r.Context(), me, status)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	views := make([]reportView, 0, len(list))
	for _, rp := range list {
		v := reportView{
			ID: rp.ID, MatchHost: rp.MatchHost, MatchID: rp.MatchID, HostedHere: rp.MatchID != "" && rp.MatchHost == h.Self,
			Category: rp.Category, Body: rp.Body, Status: rp.Status, Resolution: rp.Resolution,
			CreatedAt: rp.CreatedAt, ResolvedAt: nullTime(rp.ResolvedAt),
		}
		if v.Reporter, err = h.ref(r.Context(), rp.Reporter); err != nil {
			httpx.WriteError(w, err)
			return
		}
		if v.Target, err = h.ref(r.Context(), rp.Target); err != nil {
			httpx.WriteError(w, err)
			return
		}
		views = append(views, v)
	}
	httpx.WriteJSON(w, http.StatusOK, views)
}

func (h *HTTP) resolveReport(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if _, err := h.Service.ResolveReport(r.Context(), me, r.PathValue("id"), req.Note); err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTP) sanctionsFor(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	account := shared.AccountID(r.PathValue("id"))
	list, err := h.Service.SanctionsFor(r.Context(), me, account)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	ref, err := h.ref(r.Context(), account)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"account": ref, "sanctions": h.sanctionViews(list, true)})
}

// maxSuspensionDays は利用停止の期間（日数）の上限。これより長くするなら無期限にする。
const maxSuspensionDays = 3650

func (h *HTTP) sanction(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req struct {
		AccountID shared.AccountID    `json:"accountId"`
		Kind      domain.SanctionKind `json:"kind"`
		Reason    domain.Category     `json:"reason"`
		Note      string              `json:"note"`
		// レッドカードの期間（日数）。0 なら無期限
		Days int `json:"days"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if req.Days < 0 || req.Days > maxSuspensionDays {
		httpx.WriteError(w, shared.Invalid("期間は0〜3650日で指定してください（0 は無期限）"))
		return
	}
	var endsAt time.Time
	if req.Days > 0 {
		endsAt = h.Clock.Now().Add(time.Duration(req.Days) * 24 * time.Hour)
	}
	sc, err := h.Service.Sanction(r.Context(), me, req.AccountID, req.Kind, req.Reason, req.Note, endsAt)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, h.sanctionView(sc, true))
}

func (h *HTTP) lift(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	sc, err := h.Service.LiftSanction(r.Context(), me, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.sanctionView(sc, true))
}

func (h *HTTP) void(w http.ResponseWriter, r *http.Request) {
	me, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req struct {
		MatchID shared.MatchID `json:"matchId"`
		Reason  string         `json:"reason"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.Service.VoidMatch(r.Context(), me, req.MatchID, req.Reason); err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
