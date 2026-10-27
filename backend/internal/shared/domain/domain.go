// Package domain は全モジュールが共有する最小限のドメイン概念（共有カーネル）を定義する。
package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// AccountID はアカウントの識別子。
type AccountID string

// MatchID は対戦の識別子。
type MatchID string

// NewID はランダムな識別子を生成する。
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Clock は現在時刻を返す。テストで時刻を固定できるよう、ドメインは time.Now を直接呼ばない。
type Clock interface {
	Now() time.Time
}

// SystemClock は実際の時刻を返す Clock。
type SystemClock struct{}

// Now は現在時刻を返す。
func (SystemClock) Now() time.Time { return time.Now() }

// 共通のエラー。adapter で HTTP ステータスに変換する。
var (
	ErrNotFound     = errors.New("not found")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
)

// ValidationError は入力値の誤り。
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// Invalid は ValidationError を返す。
func Invalid(msg string) error { return &ValidationError{Message: msg} }

// ForbiddenError は理由を利用者に伝える ErrForbidden（利用停止中の操作など）。
type ForbiddenError struct {
	Message string
}

func (e *ForbiddenError) Error() string        { return e.Message }
func (e *ForbiddenError) Is(target error) bool { return target == ErrForbidden }

// Forbidden は理由付きの ErrForbidden を返す。
func Forbidden(msg string) error { return &ForbiddenError{Message: msg} }
