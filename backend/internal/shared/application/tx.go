// Package application は全モジュールのユースケースが共有するインターフェースを定義する。
package application

import "context"

// TxManager はトランザクション境界を表す。fn の中で行ったリポジトリ操作は1つのトランザクションで実行される。
type TxManager interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}
