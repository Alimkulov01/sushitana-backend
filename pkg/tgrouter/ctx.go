package tgrouter

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"sushitana/pkg/tgrouter/interfaces"
)

type ctxState struct {
	stateName *string
	data      map[string]string
}

type Ctx struct {
	update   *tgbotapi.Update
	bot      *tgbotapi.BotAPI
	handlers Handler
	index    int8
	state    *ctxState
	stateDB  interfaces.State
	Context  context.Context
}

func (c *Ctx) reset() {
	c.handlers = nil
	c.index = -1
	c.state = nil
}

func (c *Ctx) Bot() *tgbotapi.BotAPI {
	return c.bot
}

func (c *Ctx) Update() *tgbotapi.Update {
	return c.update
}

func (c *Ctx) SetState(state string, data map[string]string) {
	c.state = &ctxState{
		stateName: &state,
		data:      data,
	}
}

func (c *Ctx) State() ctxState {
	return *c.state
}

func (c *Ctx) UpdateState(state string, data map[string]string) error {
	c.SetState(state, data)
	return c.stateDB.Set(c.Context, int(c.update.FromChat().ID), int(c.update.FromChat().ID), state, data)
}

func (c *Ctx) GetState() (string, map[string]string, error) {
	return c.stateDB.Get(c.Context, int(c.update.FromChat().ID), int(c.update.FromChat().ID))
}

func (c *Ctx) ClearState() error {
	c.SetState("", nil)
	return c.stateDB.Delete(c.Context, int(c.update.FromChat().ID), int(c.update.FromChat().ID))
}

func (c *Ctx) GetStateData(key string) (string, error) {
	return c.stateDB.GetData(c.Context, int(c.update.FromChat().ID), int(c.update.FromChat().ID), key)
}

func (c *Ctx) UpdateStateData(m map[string]string) error {
	return c.stateDB.UpdateData(c.Context, int(c.update.FromChat().ID), int(c.update.FromChat().ID), m)
}

// next should be used only inside middleware.
// It executes the pending handlers in the chain inside the calling handler.
// TODO: add example here.
func (c *Ctx) next() {
	c.index++
	c.handlers(c)
}
