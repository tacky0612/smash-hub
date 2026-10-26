package domain

import (
	"bytes"
	"encoding/json"
)

// Canonical は v を署名対象として正規化した JSON にする。
// オブジェクトのキーを昇順に並べ、不要な空白を除き、HTML 用のエスケープを行わない（docs/architecture/federation.md 6.1）。
// 同じ内容なら、どのインスタンスで作っても同じバイト列になる。
func Canonical(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var generic any
	if err := dec.Decode(&generic); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(generic); err != nil { // map のキーは昇順で出力される
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
