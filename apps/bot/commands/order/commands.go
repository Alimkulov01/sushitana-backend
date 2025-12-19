package order

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"sushitana/internal/cart"
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

type (
	Params struct {
		fx.In
		Logger  logger.Logger
		CartSvc cart.Service
	}

	Commands struct {
		logger  logger.Logger
		cartSvc cart.Service
	}
)

func New(p Params) Commands {
	return Commands{
		logger:  p.Logger,
		cartSvc: p.CartSvc,
	}
}

func (c *Commands) ConfirmOrderHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}

	chatID := ctx.Update().FromChat().ID
	text := strings.TrimSpace(ctx.Update().Message.Text)

	account, ok := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if !ok || account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	if text == texts.Get(lang, texts.CartConfirm) {
		_ = ctx.UpdateState("select_delivery_type", nil) // <--
		c.Confirm(ctx)
		return
	}

	_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectFromMenu)))
}

func (c *Commands) Confirm(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID

	account, ok := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if !ok || account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	crt, err := c.cartSvc.GetByUserTgID(ctx.Context, chatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Корзина пуста"))
			return
		}
		c.logger.Error(ctx.Context, "get cart error", zap.Error(err))
		return
	}

	if len(crt.Cart.Products) == 0 {
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Корзина пуста"))
		return
	}

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectDeliveryType))
	msg.ReplyMarkup = c.deliveryTypeKeyboard(lang)
	_, _ = ctx.Bot().Send(msg)
}

func (c *Commands) DeliveryTypeHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}

	chatID := ctx.Update().FromChat().ID
	text := strings.TrimSpace(ctx.Update().Message.Text)

	account, ok := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if !ok || account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	switch {
	case sameBtnText(text, texts.Get(lang, texts.DeliveryBtn)):
		_ = ctx.UpdateState("wait_address", map[string]string{"deliveryType": "DELIVERY"})
		c.AskLocationOrAddress(ctx)
		return

	case sameBtnText(text, texts.Get(lang, texts.PickupBtn)):
		_ = ctx.UpdateState("wait_pickup_branch", map[string]string{"deliveryType": "PICKUP"})
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Выберите филиал для самовывоза:"))
		return
	}

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectDeliveryType))
	msg.ReplyMarkup = c.deliveryTypeKeyboard(lang)
	_, _ = ctx.Bot().Send(msg)
}

func (c *Commands) AskLocationOrAddress(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID

	account, ok := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if !ok || account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.AskSendLocation))
	msg.ReplyMarkup = c.locationKeyboard(lang)
	_, _ = ctx.Bot().Send(msg)
}
func (c *Commands) WaitAddressHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}

	chatID := ctx.Update().FromChat().ID

	account, ok := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if !ok || account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	txt := strings.TrimSpace(ctx.Update().Message.Text)
	if txt == texts.Get(lang, texts.BackButton) {
		_ = ctx.UpdateState("select_delivery_type", nil)
		c.Confirm(ctx)
		return
	}

	// Location keldi
	if ctx.Update().Message.Location != nil {
		lat := ctx.Update().Message.Location.Latitude
		lng := ctx.Update().Message.Location.Longitude
		_ = ctx.UpdateState("wait_payment", map[string]string{
			"deliveryType": "DELIVERY",
			"addressLat":   strconv.FormatFloat(lat, 'f', 6, 64),
			"addressLng":   strconv.FormatFloat(lng, 'f', 6, 64),
		})

		// keyboardni yig‘ishtirish
		rm := tgbotapi.NewRemoveKeyboard(true)
		msg := tgbotapi.NewMessage(chatID, "Локация получена ✅")
		msg.ReplyMarkup = rm
		_, _ = ctx.Bot().Send(msg)

		// keyingi qadam: payment tanlash (siz keyin qo‘shasiz)
		return
	}

	// Text address
	if txt != "" {
		_ = ctx.UpdateState("wait_payment", map[string]string{
			"deliveryType": "DELIVERY",
			"addressText":  txt,
		})

		rm := tgbotapi.NewRemoveKeyboard(true)
		msg := tgbotapi.NewMessage(chatID, "Адрес сохранён ✅")
		msg.ReplyMarkup = rm
		_, _ = ctx.Bot().Send(msg)

		// keyingi qadam: payment tanlash (siz keyin qo‘shasiz)
		return
	}

	// bo‘sh kelsa qayta so‘raymiz
	c.AskLocationOrAddress(ctx)
}

func (c *Commands) deliveryTypeKeyboard(lang utils.Lang) tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.DeliveryBtn)),
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.PickupBtn)),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.BackButton)),
		),
	)
	kb.ResizeKeyboard = true
	kb.OneTimeKeyboard = false
	return kb
}

func (c *Commands) locationKeyboard(lang utils.Lang) tgbotapi.ReplyKeyboardMarkup {
	locBtn := tgbotapi.NewKeyboardButtonLocation(texts.Get(lang, texts.SendLocationBtn))

	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(locBtn),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.BackButton)),
		),
	)
	kb.ResizeKeyboard = true
	kb.OneTimeKeyboard = false
	return kb
}

func sameBtnText(got, want string) bool {
	got = strings.TrimSpace(strings.ReplaceAll(got, "\uFE0F", ""))
	want = strings.TrimSpace(strings.ReplaceAll(want, "\uFE0F", ""))
	return got == want
}
