// Package terms はインスタンスの利用規約を読み込む。
// 既定ではプロジェクト共通のひな形（default.md）を使い、運営者はファイルを置いて独自の規約に差し替えられる。
package terms

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

//go:embed default.md
var defaultText string

// Terms は利用規約の本文（Markdown）と版。
type Terms struct {
	Text string
	// Version は本文の SHA-256 の先頭16文字。登録時に同意した版として記録する
	Version string
	// Custom は運営者が独自の規約に差し替えているか
	Custom bool
}

// Vars は本文の {{name}} に埋め込む値。
type Vars struct {
	InstanceName string
	PolicyURL    string
	RulesURL     string
	Contact      string // 問い合わせ先。空なら案内文を入れる
}

// Load は path（空ならひな形）から利用規約を読み込み、{{instance_name}} などを埋め込む。
func Load(path string, v Vars) (Terms, error) {
	text, custom := defaultText, false
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return Terms{}, fmt.Errorf("TERMS_FILE: %w", err)
		}
		text, custom = string(b), true
	}
	contact := v.Contact
	if contact == "" {
		contact = "問い合わせ先は運営者が本サービス上で案内します。"
	}
	text = strings.NewReplacer(
		"{{instance_name}}", v.InstanceName,
		"{{policy_url}}", v.PolicyURL,
		"{{rules_url}}", v.RulesURL,
		"{{contact}}", contact,
	).Replace(text)
	sum := sha256.Sum256([]byte(text))
	return Terms{Text: text, Version: hex.EncodeToString(sum[:])[:16], Custom: custom}, nil
}
