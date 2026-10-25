// Package database は PostgreSQL への接続・トランザクション・マイグレーションを提供する。
package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier は pgxpool.Pool と pgx.Tx の共通部分。
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// DB は接続プール。application.TxManager を実装する。
type DB struct {
	Pool *pgxpool.Pool
}

// Open は接続プールを作成し、疎通を確認する。
func Open(ctx context.Context, url string) (*DB, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &DB{Pool: pool}, nil
}

// Close は接続プールを閉じる。
func (d *DB) Close() { d.Pool.Close() }

type txKey struct{}

// InTx は fn を1つのトランザクションで実行する。既にトランザクション中ならそれを使う。
func (d *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Q は ctx にトランザクションがあればそれを、なければ接続プールを返す。
func (d *DB) Q(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return d.Pool
}

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate は未適用のマイグレーションをファイル名順に適用する。
func (d *DB) Migrate(ctx context.Context) error {
	if _, err := d.Pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return err
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		err := d.InTx(ctx, func(ctx context.Context) error {
			q := d.Q(ctx)
			// 複数プロセスが同時に起動しても二重に適用しないようにする
			if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(727001)`); err != nil {
				return err
			}
			var exists bool
			if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&exists); err != nil {
				return err
			}
			if exists {
				return nil
			}
			sql, err := migrations.ReadFile(name)
			if err != nil {
				return err
			}
			if _, err := q.Exec(ctx, string(sql)); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			_, err = q.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}
