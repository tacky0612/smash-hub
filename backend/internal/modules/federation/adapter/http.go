package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/application"
	"github.com/tacky0612/smash-hub/backend/internal/platform/httpx"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// HTTP は連合の API（/federation/*）のハンドラ。
type HTTP struct {
	Service     *application.Service
	Matchmaking application.InboundMatchmaking
	Matches     application.InboundMatches
}

// Register はルートを登録する。
func (h *HTTP) Register(mux *http.ServeMux) {
	// 公開の API（誰でも取得・検証できる）
	mux.HandleFunc("GET /federation/instance", h.instance)
	mux.HandleFunc("GET /federation/log", h.log)
	// 連合先からの署名付きリクエストだけを受け付ける API
	mux.Handle("POST /federation/match-requests", h.signed(h.announce))
	mux.Handle("DELETE /federation/match-requests/{id}", h.signed(h.withdraw))
	mux.Handle("POST /federation/match-claims", h.signed(h.claim))
	mux.Handle("GET /federation/matches/{id}", h.signed(h.live))
	mux.Handle("POST /federation/match-reports", h.signed(h.report))
}

type peerKey struct{}

// signed は、連合先（許可リスト）のインスタンスの鍵で署名されたリクエストだけを通す。
func (h *HTTP) signed(next func(w http.ResponseWriter, r *http.Request, body []byte)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
		if err != nil {
			httpx.WriteError(w, shared.Invalid("リクエストが大きすぎます"))
			return
		}
		peer := r.Header.Get(HeaderInstance)
		date, err := time.Parse(time.RFC3339, r.Header.Get(HeaderDate))
		if err != nil || !h.Service.IsPeer(peer) {
			httpx.WriteError(w, shared.ErrForbidden)
			return
		}
		if d := time.Since(date); d > MaxClockSkew || d < -MaxClockSkew {
			httpx.WriteError(w, shared.ErrForbidden)
			return
		}
		pub, err := h.Service.PeerPublicKey(r.Context(), peer)
		if err != nil || !VerifyKey(pub, SigningString(r.Method, r.URL.RequestURI(), r.Header.Get(HeaderDate), body), r.Header.Get(HeaderSignature)) {
			httpx.WriteError(w, shared.ErrForbidden)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), peerKey{}, peer))
		next(w, r, body)
	})
}

func peerOf(r *http.Request) string { s, _ := r.Context().Value(peerKey{}).(string); return s }

func decode(body []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return shared.Invalid("リクエストの形式が正しくありません")
	}
	return nil
}

func (h *HTTP) instance(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, h.Service.Info())
}

func (h *HTTP) log(w http.ResponseWriter, r *http.Request) {
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := h.Service.OwnLog(r.Context(), since, limit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"instance_id": h.Service.Self, "events": events})
}

func (h *HTTP) announce(w http.ResponseWriter, r *http.Request, body []byte) {
	var req application.RemoteRequest
	if err := decode(body, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.Service.ReceiveRequest(r.Context(), peerOf(r), req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTP) withdraw(w http.ResponseWriter, r *http.Request, _ []byte) {
	if err := h.Service.WithdrawRequest(r.Context(), peerOf(r), r.PathValue("id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTP) claim(w http.ResponseWriter, r *http.Request, body []byte) {
	var c application.Claim
	if err := decode(body, &c); err != nil {
		httpx.WriteError(w, err)
		return
	}
	id, err := h.Matchmaking.AcceptClaim(r.Context(), peerOf(r), c)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"match_id": id})
}

func (h *HTTP) live(w http.ResponseWriter, r *http.Request, _ []byte) {
	m, err := h.Matches.Live(r.Context(), peerOf(r), shared.MatchID(r.PathValue("id")))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, m)
}

func (h *HTTP) report(w http.ResponseWriter, r *http.Request, body []byte) {
	var rep application.RemoteReport
	if err := decode(body, &rep); err != nil {
		httpx.WriteError(w, err)
		return
	}
	m, err := h.Matches.Report(r.Context(), peerOf(r), rep)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, m)
}

// IsPeerStatus は連合先のエラーが指定のステータスかを返す。
func IsPeerStatus(err error, status int) bool {
	var pe *PeerError
	return errors.As(err, &pe) && pe.Status == status
}
