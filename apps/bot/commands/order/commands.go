package order

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sushitana/internal/cart"
	"sushitana/internal/structs"
	"sushitana/internal/texts"
	"sushitana/pkg/logger"
	"sushitana/pkg/tgrouter"
	"sushitana/pkg/utils"
	"sushitana/pkg/utils/ctxman"
	"unicode"

	tgbotapi "github.com/ilpy20/telegram-bot-api/v7"
	"github.com/spf13/cast"
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
	case text == texts.Get(lang, texts.DeliveryBtn):
		_ = ctx.UpdateState("wait_address", map[string]string{"deliveryType": "DELIVERY"})
		c.AskLocationOrAddress(ctx)
		return

	case text == texts.Get(lang, texts.PickupBtn):
		_, data, _ := ctx.GetState()
		if data == nil {
			data = map[string]string{}
		}
		data["deliveryType"] = "PICKUP"

		_ = ctx.UpdateState("checkout_preview", data)
		c.ShowCheckoutPreview(ctx)
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

	txt := ctx.Update().Message.Text

	if txt == texts.Get(lang, texts.BackButton) {
		data := keepData(ctx)
		_ = ctx.UpdateState("select_delivery_type", data)
		c.Confirm(ctx)
		return
	}
	if ctx.Update().Message.Location != nil {
		lat := ctx.Update().Message.Location.Latitude
		lng := ctx.Update().Message.Location.Longitude
		_ = ctx.UpdateState("wait_payment", map[string]string{
			"deliveryType": "DELIVERY",
			"addressLat":   strconv.FormatFloat(lat, 'f', 6, 64),
			"addressLng":   strconv.FormatFloat(lng, 'f', 6, 64),
		})

		rm := tgbotapi.NewRemoveKeyboard(true)
		msg := tgbotapi.NewMessage(chatID, "Локация получена ✅")
		msg.ReplyMarkup = rm
		_, _ = ctx.Bot().Send(msg)

		return
	}

	if txt != "" {
		_ = ctx.UpdateState("wait_payment", map[string]string{
			"deliveryType": "DELIVERY",
			"addressText":  txt,
		})

		rm := tgbotapi.NewRemoveKeyboard(true)
		msg := tgbotapi.NewMessage(chatID, "Адрес сохранён ✅")
		msg.ReplyMarkup = rm
		_, _ = ctx.Bot().Send(msg)

		return
	}

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

func normBtn(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "\uFE0F", "")

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func eqBtn(got, want string) bool {
	return normBtn(got) == normBtn(want)
}

func keepData(ctx *tgrouter.Ctx) map[string]string {
	_, data, _ := ctx.GetState()
	if data == nil {
		data = map[string]string{}
	}
	return data
}

func (c *Commands) ShowCheckoutPreview(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID

	account, ok := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if !ok || account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	_, data, _ := ctx.GetState()
	if data == nil {
		data = map[string]string{}
	}

	crt, err := c.cartSvc.GetByUserTgID(ctx.Context, chatID)
	if err != nil {
		c.logger.Error(ctx.Context, "get cart error", zap.Error(err))
		return
	}
	if len(crt.Cart.Products) == 0 {
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Корзина пуста"))
		return
	}

	dt := strings.ToUpper(data["deliveryType"])
	orderTypeLine := texts.Get(lang, texts.OrderTypeDelivery)
	if dt == "PICKUP" {
		orderTypeLine = texts.Get(lang, texts.OrderTypePickup)
	}

	addrLine := ""
	if dt == "DELIVERY" {
		if at := strings.TrimSpace(data["addressText"]); at != "" {
			addrLine = "📍 Manzil: " + at + "\n"
		} else if data["addressLat"] != "" && data["addressLng"] != "" {
			addrLine = fmt.Sprintf("📍 Lokatsiya: %s,%s\n", data["addressLat"], data["addressLng"])
		}
	}

	var b strings.Builder
	b.WriteString(texts.Get(lang, texts.OrderPreviewTitle))
	b.WriteString(fmt.Sprintf(texts.Get(lang, texts.OrderPreviewName), account.Name))
	b.WriteString(fmt.Sprintf(texts.Get(lang, texts.OrderPreviewPhone), account.Phone))
	b.WriteString(orderTypeLine + "\n")
	if addrLine != "" {
		b.WriteString(addrLine)
	}
	b.WriteString("\n")

	var total float64
	for i, p := range crt.Cart.Products {
		prodName := nameByLang(p.Name, string(lang))
		line := fmt.Sprintf("%d. %s\n%v x %v = %v\n\n", i+1, prodName, p.Count, p.Price, p.Count*p.Price)
		b.WriteString(line)
		total += cast.ToFloat64(p.Count * p.Price)
	}
	b.WriteString(fmt.Sprintf(texts.Get(lang, texts.OrderPreviewTotal), total))

	msg := tgbotapi.NewMessage(chatID, b.String())
	msg.ReplyMarkup = c.previewKeyboard(lang)
	_, _ = ctx.Bot().Send(msg)
}

func (c *Commands) CheckoutPreviewHandler(ctx *tgrouter.Ctx) {
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
	txt := ctx.Update().Message.Text

	// ✅ Tasdiqlash
	if txt == texts.Get(lang, texts.CartConfirm) { // ConfirmBtn key qo‘shasiz (pastda)
		rm := tgbotapi.NewRemoveKeyboard(true)
		m := tgbotapi.NewMessage(chatID, "Buyurtma qabul qilindi ✅")
		m.ReplyMarkup = rm
		_, _ = ctx.Bot().Send(m)

		// TODO: shu yerda order create logikasini chaqirasiz (DB/iiko)
		// c.orderSvc.Create(...)

		_ = ctx.UpdateState("show_main_menu", nil)
		return
	}

	// boshqa text kelsa preview’ni qayta chiqaramiz
	c.ShowCheckoutPreview(ctx)
}

func (c *Commands) previewKeyboard(lang utils.Lang) tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.CartConfirm)),
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.CancelBtn)),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.BackButton)),
		),
	)
	kb.ResizeKeyboard = true
	kb.OneTimeKeyboard = false
	return kb
}

func nameByLang(n structs.Name, lang string) string {
	switch strings.ToLower(lang) {
	case "uz":
		if n.Uz != "" {
			return n.Uz
		}
	case "ru":
		if n.Ru != "" {
			return n.Ru
		}
	case "en":
		if n.En != "" {
			return n.En
		}
	}
	if n.Ru != "" {
		return n.Ru
	}
	if n.Uz != "" {
		return n.Uz
	}
	return n.En
}
