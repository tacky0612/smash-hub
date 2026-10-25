package adapter

import (
	"errors"
	"net/http"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/modules/account/application"
	"github.com/tacky0612/smash-hub/backend/internal/modules/account/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/crypto"
	"github.com/tacky0612/smash-hub/backend/internal/platform/httpx"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// SessionCookie はセッション Cookie の名前。
const SessionCookie = "smash_hub_session"

// HTTP は account の HTTP ハンドラ。
type HTTP struct {
	Service         *application.Service
	CookieSecure    bool
	DevLoginEnabled bool
	PublicBaseURL   string
	SourceCodeURL   string
}

// AccountView はアカウントの公開情報。
type AccountView struct {
	ID          shared.AccountID `json:"id"`
	DisplayName string           `json:"displayName"`
	MainFighter shared.FighterID `json:"mainFighter"`
}

// ToView はアカウントを AccountView に変換する。
func ToView(a domain.Account) AccountView {
	return AccountView{ID: a.ID, DisplayName: a.DisplayName, MainFighter: a.MainFighter}
}

// Register はルートを登録する。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/instance", h.instance)
	mux.HandleFunc("GET /api/fighters", h.fighters)
	mux.HandleFunc("POST /api/auth/dev/login", h.devLogin)
	mux.HandleFunc("GET /api/auth/discord/start", h.discordStart)
	mux.HandleFunc("GET /api/auth/discord/callback", h.discordCallback)
	mux.HandleFunc("POST /api/auth/logout", h.logout)
	mux.HandleFunc("GET /api/me", h.me)
	mux.HandleFunc("POST /api/register", h.register)
	mux.HandleFunc("PUT /api/me/profile", h.updateProfile)
}

// Middleware はセッション Cookie を検証し、登録済みならアカウント ID を ctx に入れる。
func (h *HTTP) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(SessionCookie); err == nil {
			if sess, err := h.Service.Authenticate(r.Context(), c.Value); err == nil && sess.Registered() {
				r = r.WithContext(httpx.WithAccountID(r.Context(), sess.AccountID))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *HTTP) instance(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"name":          "Smash-hub",
		"sourceCodeUrl": h.SourceCodeURL,
		"loginProviders": map[string]bool{
			"dev":     h.DevLoginEnabled,
			"discord": h.Service.Discord != nil,
		},
	})
}

func (h *HTTP) fighters(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, shared.Fighters)
}

func (h *HTTP) devLogin(w http.ResponseWriter, r *http.Request) {
	if !h.DevLoginEnabled {
		httpx.WriteError(w, shared.ErrNotFound)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	token, err := h.Service.DevLogin(r.Context(), req.Name)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.setSession(w, token)
	h.writeMe(w, r, token)
}

func (h *HTTP) discordStart(w http.ResponseWriter, r *http.Request) {
	url, err := h.Service.StartDiscord(r.Context(), crypto.RandomToken(), crypto.RandomToken())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *HTTP) discordCallback(w http.ResponseWriter, r *http.Request) {
	token, err := h.Service.CompleteDiscord(r.Context(), r.URL.Query().Get("state"), r.URL.Query().Get("code"))
	if err != nil {
		http.Redirect(w, r, h.PublicBaseURL+"/login?error=discord", http.StatusFound)
		return
	}
	h.setSession(w, token)
	http.Redirect(w, r, h.PublicBaseURL+"/", http.StatusFound)
}

func (h *HTTP) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookie); err == nil {
		if err := h.Service.Logout(r.Context(), c.Value); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	http.SetCookie(w, h.cookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTP) me(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"loggedIn": false})
		return
	}
	h.writeMe(w, r, c.Value)
}

func (h *HTTP) writeMe(w http.ResponseWriter, r *http.Request, token string) {
	sess, err := h.Service.Authenticate(r.Context(), token)
	if errors.Is(err, shared.ErrUnauthorized) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"loggedIn": false})
		return
	}
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	me, err := h.Service.GetMe(r.Context(), sess)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	res := map[string]any{"loggedIn": true, "registered": me.Registered}
	if me.Account != nil {
		res["account"] = ToView(*me.Account)
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *HTTP) register(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		httpx.WriteError(w, shared.ErrUnauthorized)
		return
	}
	sess, err := h.Service.Authenticate(r.Context(), c.Value)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req struct {
		DisplayName string           `json:"displayName"`
		MainFighter shared.FighterID `json:"mainFighter"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	a, err := h.Service.Register(r.Context(), sess, req.DisplayName, req.MainFighter)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, ToView(a))
}

func (h *HTTP) updateProfile(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var req struct {
		DisplayName string           `json:"displayName"`
		MainFighter shared.FighterID `json:"mainFighter"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	a, err := h.Service.UpdateProfile(r.Context(), id, req.DisplayName, req.MainFighter)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ToView(a))
}

func (h *HTTP) setSession(w http.ResponseWriter, token string) {
	http.SetCookie(w, h.cookie(token, int(domain.SessionTTL/time.Second)))
}

func (h *HTTP) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}
