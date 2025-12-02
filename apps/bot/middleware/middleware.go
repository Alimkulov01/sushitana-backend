package middleware

import (
	"context"
	"errors"
	"sushitana/internal/client"
	"sushitana/internal/structs"
	"sushitana/internal/texts"
	"sushitana/pkg/logger"
	"sushitana/pkg/tgrouter"
	"sushitana/pkg/utils"
	"sushitana/pkg/utils/ctxman"

	tgbotapi "github.com/ilpy20/telegram-bot-api/v7"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Provide(New)

type Params struct {
	fx.In
	Logger    logger.Logger
	ClientSvc client.Service
}

type Middleware interface {
	AccountMw(next tgrouter.Handler) tgrouter.Handler
}

type mw struct {
	logger    logger.Logger
	clientSvc client.Service
}

func New(p Params) Middleware {
	return &mw{
		clientSvc: p.ClientSvc,
		logger:    p.Logger,
	}
}

func (m *mw) AccountMw(next tgrouter.Handler) tgrouter.Handler {
	return func(c *tgrouter.Ctx) {

		tgID := c.Update().FromChat().ID

		account, err := m.clientSvc.GetByTgID(c.Context, tgID)
		if err != nil {
			if errors.Is(err, structs.ErrNotFound) {

				m.logger.Info(c.Context, "User not found, creating new", zap.Int64("tgid", tgID))

				account, err = m.clientSvc.Create(c.Context, structs.CreateClient{TgID: tgID})
				if err != nil {
					m.logger.Error(c.Context, "failed to create account", zap.Error(err))
					_, _ = c.Bot().Send(tgbotapi.NewMessage(tgID, "Xatolik, keyinroq urinib ko‘ring"))
					return
				}

			} else {
				m.logger.Error(c.Context, "failed to get account", zap.Error(err))
				_, _ = c.Bot().Send(tgbotapi.NewMessage(tgID, texts.Get(utils.UZ, texts.Retry)))
				return
			}
		}

		c.Context = context.WithValue(c.Context, ctxman.AccountKey{}, &account)

		typing := tgbotapi.NewChatAction(tgID, tgbotapi.ChatTyping)
		c.Bot().Send(typing)

		next(c)
	}
}
