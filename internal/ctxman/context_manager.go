package ctxman

import "context"

type (
	UserCtx struct{}
)

func Get[T any](ctx context.Context, key any) (T, bool) {
	var result T

	value := ctx.Value(key)
	if value == nil {
		return result, false
	}

	result, ok := value.(T)
	return result, ok
}

func Has[T any](ctx context.Context, key any) bool {
	value := ctx.Value(key)
	return value != nil
}
