package interfaces

import (
	"context"
)

type State interface {
	Set(ctx context.Context, userId int, chatId int, state string, data map[string]string) error
	Get(ctx context.Context, userId int, chatId int) (string, map[string]string, error)
	Delete(ctx context.Context, userId int, chatId int) error
	GetData(ctx context.Context, userId, chatId int, key string) (string, error)
	UpdateData(ctx context.Context, userId, chatId int, data map[string]string) error
}
