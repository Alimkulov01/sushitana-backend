package tgrouter

import (
	"log/slog"

	"sushitana/pkg/tgrouter/callback"
)

type FilterType interface {
	MessageFilter | CommandFilter | StickerFilter | StateFilter | CallbackFilter | any
}

type (
	MessageFilter  struct{}
	CommandFilter  struct{}
	StickerFilter  struct{}
	StateFilter    struct{}
	CallbackFilter struct{}
)

type Filter[F FilterType] func(*Ctx) bool

func Message() Filter[MessageFilter] {
	return func(c *Ctx) bool {
		return c.update.Message != nil
	}
}

func Command() Filter[CommandFilter] {
	return func(c *Ctx) bool {
		return c.update.Message.IsCommand()
	}
}

func Cmd(cmd string) Filter[CommandFilter] {
	return func(c *Ctx) bool {
		return c.update.Message != nil && c.update.Message.IsCommand() && c.update.Message.Command() == cmd
	}
}

func Callback(query string) Filter[CallbackFilter] {
	return func(c *Ctx) bool {
		if c.update.CallbackQuery == nil {
			return false
		}

		key := callback.Query(c.update.CallbackQuery.Data)

		slog.Info("callback", "key", key, "query", query)

		return key == query
	}
}

func Sticker() Filter[StickerFilter] {
	return func(c *Ctx) bool {
		return c.update.Message != nil && c.update.Message.Sticker != nil
	}
}

func State(name string) Filter[StateFilter] {
	return func(c *Ctx) bool {
		if c.update.Message == nil {
			return false
		}

		if c.state == nil {
			s, data, err := c.GetState()
			if err == nil {
				c.SetState(s, data)
			}
		}
		return c.state != nil && *c.state.stateName == name
	}
}

func PreCheckout() Filter[MessageFilter] {
	return func(c *Ctx) bool {
		return c.update.PreCheckoutQuery != nil
	}
}

func Any() Filter[any] {
	return func(c *Ctx) bool {
		return true
	}
}

func OnPayment() Filter[any] {
	return func(c *Ctx) bool {
		return c.update.Message != nil && c.update.Message.SuccessfulPayment != nil
	}
}
