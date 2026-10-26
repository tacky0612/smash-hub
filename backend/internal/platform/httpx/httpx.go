// Package httpx は HTTP ハンドラで共通に使う JSON 入出力・エラー変換・ミドルウェアを提供する。
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// CSRFHeader は状態を変更するリクエストに必須のヘッダー。
// 独自ヘッダーはクロスサイトの単純なリクエストでは付与できないため、SameSite Cookie と合わせて CSRF を防ぐ。
const CSRFHeader = "X-Smash-Hub"

// WriteJSON は v を JSON で書き出す。
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ReadJSON はリクエストボディを v に読み込む（最大 64KB）。
func ReadJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return domain.Invalid("リクエストの形式が正しくありません")
	}
	return nil
}

// WriteError はエラーを HTTP ステータスと JSON に変換して書き出す。
func WriteError(w http.ResponseWriter, err error) {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid", "message": ve.Message})
	case errors.Is(err, domain.ErrUnauthorized):
		WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "message": "ログインが必要です"})
	case errors.Is(err, domain.ErrForbidden):
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "この操作はできません"})
	case errors.Is(err, domain.ErrNotFound):
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "見つかりません"})
	case errors.Is(err, domain.ErrConflict):
		WriteJSON(w, http.StatusConflict, map[string]string{"error": "conflict", "message": err.Error()})
	default:
		slog.Error("internal error", "err", err)
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal", "message": "サーバーでエラーが発生しました"})
	}
}

type accountKey struct{}

// WithAccountID はログイン中のアカウント ID を ctx に格納する。
func WithAccountID(ctx context.Context, id domain.AccountID) context.Context {
	return context.WithValue(ctx, accountKey{}, id)
}

// AccountID は ctx からログイン中のアカウント ID を取り出す。登録済みのアカウントがなければ ErrUnauthorized。
func AccountID(ctx context.Context) (domain.AccountID, error) {
	id, ok := ctx.Value(accountKey{}).(domain.AccountID)
	if !ok || id == "" {
		return "", domain.ErrUnauthorized
	}
	return id, nil
}

// CSRF は GET / HEAD 以外のリクエストに CSRFHeader を要求する。
// インスタンス間の API（/federation/）はブラウザからではなく、インスタンスの署名で検証するため対象外。
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !strings.HasPrefix(r.URL.Path, "/federation/") && r.Header.Get(CSRFHeader) == "" {
			WriteJSON(w, http.StatusForbidden, map[string]string{"error": "csrf", "message": "不正なリクエストです"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders は API レスポンスに共通のセキュリティヘッダーを付ける。
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
