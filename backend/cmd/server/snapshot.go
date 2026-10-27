package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/tacky0612/smash-hub/backend/internal/bootstrap"
	"github.com/tacky0612/smash-hub/backend/internal/platform/config"
	"github.com/tacky0612/smash-hub/backend/internal/platform/database"
)

// runSnapshot はログのスナップショットを書き出す。サーバーと同じ環境変数で DB に接続する。
func runSnapshot(args []string) error {
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	out := fs.String("o", "", "書き出すファイル（省略すると標準出力）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	app, err := bootstrap.New(cfg, db, nil)
	if err != nil {
		return err
	}
	if *out == "" {
		return app.WriteSnapshot(ctx, os.Stdout)
	}
	// 途中で失敗したときに、不完全なファイルを残さない
	tmp := *out + ".partial"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if err := app.WriteSnapshot(ctx, f); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, *out)
}

// keyFlags は -key URL=鍵 を繰り返し受け取る。
type keyFlags map[string]string

func (k keyFlags) String() string { return "" }

func (k keyFlags) Set(v string) error {
	id, key, ok := strings.Cut(v, "=")
	if !ok || id == "" || key == "" {
		return errors.New("-key は インスタンスのURL=公開鍵 の形で指定してください")
	}
	k[strings.TrimRight(id, "/")] = key
	return nil
}

// runVerifySnapshot はスナップショットを検証し、ログから再計算したレートがスナップショットのレートと一致するかを確かめる。
// 一致しなければエラーを返す（終了コード 1）。
func runVerifySnapshot(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("verify-snapshot", flag.ContinueOnError)
	keys := keyFlags{}
	fs.Var(keys, "key", "信頼できる経路で確かめたインスタンスの公開鍵（URL=鍵。繰り返し指定できる）")
	csvOut := fs.String("csv", "", "再計算した成績を CSV で書き出すファイル")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("検証するスナップショットのファイルを1つ指定してください（- で標準入力）")
	}
	in := os.Stdin
	if name := fs.Arg(0); name != "-" {
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}
	report, err := bootstrap.VerifySnapshot(context.Background(), in, keys)
	if err != nil {
		return err
	}
	h := report.Header
	fmt.Fprintf(stdout, "スナップショット: %s（作成 %s）\n", h.InstanceID, h.CreatedAt)
	fmt.Fprintln(stdout, "ログ（署名・連番・ハッシュチェーンを検証しました）:")
	for _, c := range h.Chains {
		mark := ""
		if _, ok := keys[c.InstanceID]; ok {
			mark = "（指定した鍵と一致）"
		}
		fmt.Fprintf(stdout, "  %s  seq 1〜%d  公開鍵 %s%s\n", c.InstanceID, c.Seq, c.PublicKey, mark)
	}
	fmt.Fprintf(stdout, "対戦記録: %d 件（シーズン %s のレート計算に使った記録: %d 件）\n", report.Records, h.Season.ID, report.Effective)
	if *csvOut != "" {
		if err := writeStandingsCSV(*csvOut, report.Computed); err != nil {
			return err
		}
	}
	if len(report.Mismatches) > 0 {
		for _, m := range report.Mismatches {
			fmt.Fprintln(stdout, "  食い違い:", m)
		}
		return fmt.Errorf("再計算したレートがスナップショットのレートと %d 件食い違っています", len(report.Mismatches))
	}
	fmt.Fprintf(stdout, "レート: %d 人。再計算した結果はスナップショットのレートと一致しました\n", len(report.Computed))
	return nil
}

func writeStandingsCSV(name string, standings []bootstrap.SnapshotStanding) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"account_id", "rating", "matches", "wins", "losses"})
	for _, s := range standings {
		_ = w.Write([]string{string(s.AccountID), strconv.Itoa(s.Rating), strconv.Itoa(s.Matches), strconv.Itoa(s.Wins), strconv.Itoa(s.Losses)})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
