package terms

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_DefaultFillsVariables(t *testing.T) {
	tos, err := Load("", Vars{InstanceName: "テスト", PolicyURL: "https://example.com/policy", RulesURL: "https://example.com/rules"})
	if err != nil {
		t.Fatal(err)
	}
	if tos.Custom || !strings.HasPrefix(tos.Text, "# テスト 利用規約") || strings.Contains(tos.Text, "{{") {
		t.Fatalf("埋め込みが正しくない: %q", tos.Text[:80])
	}
	if !strings.Contains(tos.Text, "https://example.com/policy") || !strings.Contains(tos.Text, "問い合わせ先は運営者が") {
		t.Error("URL・問い合わせ先の案内が入っていない")
	}
	if len(tos.Version) != 16 {
		t.Errorf("版 = %q", tos.Version)
	}
	// 本文が変われば版も変わる
	other, _ := Load("", Vars{InstanceName: "別のインスタンス"})
	if other.Version == tos.Version {
		t.Error("本文が違うのに版が同じ")
	}
}

func TestLoad_CustomFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "terms.md")
	if err := os.WriteFile(path, []byte("# {{instance_name}} の規約\n\n連絡先: {{contact}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tos, err := Load(path, Vars{InstanceName: "A", Contact: "Discord サーバー"})
	if err != nil {
		t.Fatal(err)
	}
	if !tos.Custom || tos.Text != "# A の規約\n\n連絡先: Discord サーバー\n" {
		t.Errorf("tos = %+v", tos)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.md"), Vars{}); err == nil {
		t.Error("ファイルがなくてもエラーにならない")
	}
}
