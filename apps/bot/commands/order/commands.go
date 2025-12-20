package order

import (
	"database/sql"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"

	tgbotapi "github.com/ilpy20/telegram-bot-api/v7"
	"github.com/spf13/cast"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"sushitana/internal/cart"
	"sushitana/internal/delivery"
	"sushitana/internal/order"
	"sushitana/internal/payment/click"
	"sushitana/internal/payment/payme"
	"sushitana/internal/structs"
	"sushitana/internal/texts"
	"sushitana/pkg/logger"
	"sushitana/pkg/tgrouter"
	"sushitana/pkg/utils"
	"sushitana/pkg/utils/ctxman"
)

var Module = fx.Provide(New)

type Commands struct {
	logger   logger.Logger
	cartSvc  cart.Service
	orderSvc order.Service
	clickSvc click.Service
	paymeSvc payme.Service
}

type Params struct {
	fx.In
	Logger   logger.Logger
	CartSvc  cart.Service
	OrderSvc order.Service
	ClickSvc click.Service
	PaymeSvc payme.Service
}

func New(p Params) Commands {
	return Commands{
		logger:   p.Logger,
		cartSvc:  p.CartSvc,
		orderSvc: p.OrderSvc,
		clickSvc: p.ClickSvc,
		paymeSvc: p.PaymeSvc,
	}
}

func keepData(ctx *tgrouter.Ctx) map[string]string {
	_, st, _ := ctx.GetState()
	if st == nil {
		return map[string]string{}
	}
	cp := make(map[string]string, len(st))
	for k, v := range st {
		cp[k] = v
	}
	return cp
}

// 1) Cart Confirm tugmasi bosilganda
func (c *Commands) ConfirmOrderHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}

	chatID := ctx.Update().FromChat().ID
	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}
	lang := account.Language

	txt := strings.TrimSpace(ctx.Update().Message.Text)
	if txt != texts.Get(lang, texts.CartConfirm) {
		return
	}

	_ = ctx.UpdateState("select_delivery_type", nil)
	c.Confirm(ctx)

	_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.OrderDeliveryTypeChoose)))
}

// 2) DeliveryType tanlash
func (c *Commands) Confirm(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID

	account, ok := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if !ok || account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	// Cart tgID bilan olinishi kerak
	crt, err := c.cartSvc.GetByUserTgID(ctx.Context, account.TgID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.CartEmpty)))
			return
		}
		c.logger.Error(ctx.Context, "failed to get cart", zap.Error(err))
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}
	if len(crt.Cart.Products) == 0 {
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.CartEmpty)))
		return
	}

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.OrderDeliveryTypeChoose))
	msg.ReplyMarkup = deliveryTypeKeyboard(lang)
	_, _ = ctx.Bot().Send(msg)
}

func (c *Commands) DeliveryTypeHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}

	chatID := ctx.Update().FromChat().ID
	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}
	lang := account.Language

	txt := strings.TrimSpace(ctx.Update().Message.Text)

	// back -> main menu
	if txt == texts.Get(lang, texts.BackButton) {
		rm := tgbotapi.NewMessage(chatID, "\u200b")
		rm.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		_, _ = ctx.Bot().Send(rm)

		_ = ctx.UpdateState("show_main_menu", nil)
		return
	}

	switch txt {
	case texts.Get(lang, texts.DeliveryBtn):
		data := keepData(ctx)
		data["deliveryType"] = "DELIVERY"
		// address / location keylar shu state’da to‘planadi
		_ = ctx.UpdateState("wait_address", data)
		c.AskLocationOrAddress(ctx)
		return

	case texts.Get(lang, texts.PickupBtn):
		data := keepData(ctx)
		data["deliveryType"] = "PICKUP"
		data["deliveryPrice"] = "0"
		data["distanceKm"] = "0"
		_ = ctx.UpdateState("checkout_preview", data)
		c.ShowCheckoutPreview(ctx)
		return

	default:
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectFromMenu)))
	}
}

func (c *Commands) AskLocationOrAddress(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID
	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}
	lang := account.Language

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.AskSendLocation))
	msg.ReplyMarkup = locationKeyboard(lang)
	_, _ = ctx.Bot().Send(msg)
}

// 3) Delivery bo‘lsa: address + location shu handler’da
func (c *Commands) WaitAddressHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}

	chatID := ctx.Update().FromChat().ID
	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}
	lang := account.Language

	txt := strings.TrimSpace(ctx.Update().Message.Text)

	// back -> deliveryType tanlash
	if txt == texts.Get(lang, texts.BackButton) {
		data := keepData(ctx)
		_ = ctx.UpdateState("select_delivery_type", data)
		c.Confirm(ctx)
		return
	}

	_, st, _ := ctx.GetState()
	if st == nil {
		st = map[string]string{}
	}

	// 3.1) Location keldi -> delivery price hisoblaymiz -> preview
	if ctx.Update().Message.Location != nil {
		lat := ctx.Update().Message.Location.Latitude
		lng := ctx.Update().Message.Location.Longitude

		addressText := strings.TrimSpace(st["addressText"])
		if addressText == "" {
			addressText = "geo"
		}

		info := delivery.GetDeliveryInfo(lat, lng, addressText)
		if !info.Available {
			_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, info.Reason))
			// state o‘zgarmaydi, yana lokatsiya kutamiz
			return
		}

		data := keepData(ctx)
		data["deliveryType"] = "DELIVERY"
		data["addressLat"] = strconv.FormatFloat(lat, 'f', 6, 64)
		data["addressLng"] = strconv.FormatFloat(lng, 'f', 6, 64)
		data["addressText"] = addressText
		data["deliveryPrice"] = strconv.FormatInt(info.Price, 10)
		data["distanceKm"] = strconv.FormatFloat(info.DistanceKm, 'f', 2, 64)

		_ = ctx.UpdateState("checkout_preview", data)
		c.ShowCheckoutPreview(ctx)
		return
	}

	// 3.2) User address text yubordi -> saqlaymiz, lekin lokatsiya so‘raymiz
	if txt != "" {
		data := keepData(ctx)
		data["deliveryType"] = "DELIVERY"
		data["addressText"] = txt

		// lokatsiya hali yo‘q bo‘lsa, wait_address’da qolamiz
		if strings.TrimSpace(data["addressLat"]) == "" || strings.TrimSpace(data["addressLng"]) == "" {
			_ = ctx.UpdateState("wait_address", data)
			_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.OrderAddressSavedSendLocation)))
			c.AskLocationOrAddress(ctx)
			return
		}

		// agar oldin lokatsiya bor bo‘lsa (kam uchraydi), preview’ga o‘tamiz
		_ = ctx.UpdateState("checkout_preview", data)
		c.ShowCheckoutPreview(ctx)
		return
	}
}

// 4) Preview ko‘rsatish
func (c *Commands) ShowCheckoutPreview(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}
	lang := account.Language

	crt, err := c.cartSvc.GetByUserTgID(ctx.Context, account.TgID)
	if err != nil {
		c.logger.Error(ctx.Context, "failed to get cart", zap.Error(err))
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	_, st, _ := ctx.GetState()
	if st == nil {
		st = map[string]string{}
	}

	deliveryType := strings.ToUpper(strings.TrimSpace(st["deliveryType"]))
	if deliveryType == "" {
		deliveryType = "DELIVERY"
	}

	var productsTotal float64
	var b strings.Builder

	// header
	if strings.ToLower(string(lang)) == "uz" {
		b.WriteString("🛒 Savat:\n\n")
	} else {
		b.WriteString("🛒 Корзина:\n\n")
	}

	// items
	for i, p := range crt.Cart.Products {
		name := nameByLang(p.Name, string(lang))
		lineTotal := float64(p.Count) * float64(p.Price)
		productsTotal += lineTotal

		fmt.Fprintf(&b, "%d. %s\n   %d x %s = %s %s\n\n",
			i+1,
			name,
			p.Count,
			utils.FCurrency(cast.ToFloat64(p.Price)),
			utils.FCurrency(lineTotal),
			texts.Get(lang, texts.CurrencySymbol),
		)
	}

	// delivery price
	var deliveryPrice float64
	if deliveryType == "DELIVERY" {
		if v := strings.TrimSpace(st["deliveryPrice"]); v != "" {
			deliveryPrice = cast.ToFloat64(v)
		}
		// dist := strings.TrimSpace(st["distanceKm"])

		if strings.ToLower(string(lang)) == "uz" {
			fmt.Fprintf(&b, "🚚 Yetkazib berish: %s %s\n", utils.FCurrency(deliveryPrice), texts.Get(lang, texts.CurrencySymbol))
			// if dist != "" && dist != "0" {
			// 	fmt.Fprintf(&b, "📍 Masofa: %s km\n", dist)
			// }
		} else {
			fmt.Fprintf(&b, "🚚 Доставка: %s %s\n", utils.FCurrency(deliveryPrice), texts.Get(lang, texts.CurrencySymbol))
			// if dist != "" && dist != "0" {
			// 	fmt.Fprintf(&b, "📍 Расстояние: %s km\n", dist)
			// }
		}
	}

			// sizning OrderProduct struct'ingizda yana fieldlar bo'lsa (imgUrl, boxId, etc)
			// shu yerda to'ldirasiz:
			// ImgUrl: it.ImgUrl,
			// BoxID:  it.BoxId,
	grand := productsTotal + deliveryPrice

	// total
	if strings.ToLower(string(lang)) == "uz" {
		fmt.Fprintf(&b, "\n🧾 Jami: %s %s", utils.FCurrency(grand), texts.Get(lang, texts.CurrencySymbol))
	} else {
		fmt.Fprintf(&b, "\n🧾 Итого: %s %s", utils.FCurrency(grand), texts.Get(lang, texts.CurrencySymbol))
	}

	msg := tgbotapi.NewMessage(chatID, b.String())
	msg.ReplyMarkup = c.previewKeyboard(lang)
	_, _ = ctx.Bot().Send(msg)
}

// 5) Preview’da Confirm -> payment tanlashga o‘tamiz
func (c *Commands) CheckoutPreviewHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}

	chatID := ctx.Update().FromChat().ID
	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}
	lang := account.Language

	txt := strings.TrimSpace(ctx.Update().Message.Text)

	// Cancel -> main menu (order flow cancel)
	if txt == texts.Get(lang, texts.CancelBtn) {
		rm := tgbotapi.NewMessage(chatID, "\u200b")
		rm.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		_, _ = ctx.Bot().Send(rm)

		_ = ctx.UpdateState("show_main_menu", nil)
		return
	}

	// Back -> agar delivery bo‘lsa address/location bosqichiga qaytamiz, pickup bo‘lsa deliveryType tanlashga
	if txt == texts.Get(lang, texts.BackButton) {
		_, st, _ := ctx.GetState()
		deliveryType := strings.ToUpper(strings.TrimSpace(st["deliveryType"]))
		data := keepData(ctx)

		if deliveryType == "DELIVERY" {
			_ = ctx.UpdateState("wait_address", data)
			c.AskLocationOrAddress(ctx)
		} else {
			_ = ctx.UpdateState("select_delivery_type", data)
			c.Confirm(ctx)
		}
		return
	}

	// Confirm -> payment method tanlash
	if txt == texts.Get(lang, texts.CartConfirm) {
		data := keepData(ctx)
		_ = ctx.UpdateState("select_payment_method", data)

		m := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.OrderChoosePaymentMethod))
		m.ReplyMarkup = paymentMethodKeyboard(lang)
		_, _ = ctx.Bot().Send(m)
		return
	}

	// boshqa text kelsa preview’ni qayta chiqaramiz
	c.ShowCheckoutPreview(ctx)
}

// 6) Payment tanlash -> order create -> cash bo‘lsa yakun, online bo‘lsa payment URL
func (c *Commands) SelectPaymentMethodHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}

	chatID := ctx.Update().FromChat().ID

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}
	lang := account.Language

	txt := strings.TrimSpace(ctx.Update().Message.Text)

	// state
	_, st, _ := ctx.GetState()
	if st == nil {
		st = map[string]string{}
	}

	// Back -> preview
	if eqBtn(txt, texts.Get(lang, texts.BackButton)) {
		_ = ctx.UpdateState("checkout_preview", st)
		c.ShowCheckoutPreview(ctx)
		return
	}

	// payment method parse (button text -> enum)
	var paymentMethod string
	switch txt {
	case "💵 Naqt":
		paymentMethod = "CASH"
	case "💳 Payme":
		paymentMethod = "PAYME"
	case "💳 Click":
		paymentMethod = "CLICK"
	default:
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.OrderChoosePaymentMethod)))
		return
	}

	// cart
	crt, err := c.cartSvc.GetByUserTgID(ctx.Context, account.TgID)
	if err != nil || len(crt.Cart.Products) == 0 {
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.CartEmpty)))
		return
	}

	// delivery type
	deliveryType := strings.ToUpper(strings.TrimSpace(st["deliveryType"]))
	if deliveryType == "" {
		deliveryType = "DELIVERY"
	}

	// delivery price
	var deliveryPrice int64
	if deliveryType == "DELIVERY" {
		deliveryPrice = cast.ToInt64(strings.TrimSpace(st["deliveryPrice"]))
	}

	// address
	var addr *structs.Address
	if deliveryType == "DELIVERY" {
		latStr := strings.TrimSpace(st["addressLat"])
		lngStr := strings.TrimSpace(st["addressLng"])
		name := strings.TrimSpace(st["addressText"])

		if latStr == "" || lngStr == "" {
			_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Lokatsiyani yuboring 📍"))
			return
		}

		lat, err1 := strconv.ParseFloat(latStr, 64)
		lng, err2 := strconv.ParseFloat(lngStr, 64)
		if err1 != nil || err2 != nil {
			_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Lokatsiya noto‘g‘ri. Qaytadan yuboring."))
			return
		}

		addr = &structs.Address{Lat: lat, Lng: lng, Name: name}
	}

	// create order (Create() MUST return orderID)
	req := structs.CreateOrder{
		TgID:          account.TgID,
		DeliveryType:  deliveryType,
		PaymentMethod: paymentMethod,
		Address:       addr,
		Comment:       strings.TrimSpace(st["comment"]),
		DeliveryPrice: deliveryPrice,
		Products:      toOrderProducts(crt.Cart.Products),
	}

	orderID, err := c.orderSvc.Create(ctx.Context, req)
	if err != nil {
		c.logger.Error(ctx.Context, "create order failed", zap.Error(err))
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	// CASH: clear cart and finish
	if paymentMethod == "CASH" {
		_ = c.cartSvc.Clear(ctx.Context, account.TgID)

		m := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.OrderAcceptedWaitOperator))
		m.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		_, _ = ctx.Bot().Send(m)

		_ = ctx.UpdateState("show_main_menu", nil)
		return
	}

	// PAYME / CLICK: take payment_url from order
	ord, err := c.orderSvc.GetByID(ctx.Context, orderID)
	if err != nil {
		c.logger.Error(ctx.Context, "get order after create failed", zap.Error(err), zap.String("order_id", orderID))
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	payURL := strings.TrimSpace(ord.Order.PaymentUrl)
	if payURL == "" {
		c.logger.Error(ctx.Context, "payment_url is empty after create", zap.String("order_id", orderID), zap.String("pm", paymentMethod))
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	text := paymentDetailsHTML(ord)

	btnText := "To‘lash"
	if paymentMethod == "PAYME" {
		btnText = "Payme orqali to‘lash"
	} else {
		btnText = "Click orqali to‘lash"
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL(btnText, payURL),
		),
	)

	// ✅ one message: HTML + button
	m := tgbotapi.NewMessage(chatID, text)
	m.ParseMode = "HTML"
	m.ReplyMarkup = kb
	_, _ = ctx.Bot().Send(m)

	// remove reply keyboard (optional)
	rm := tgbotapi.NewMessage(chatID, "\u200b")
	rm.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	_, _ = ctx.Bot().Send(rm)

	_ = ctx.UpdateState("waiting_payment", map[string]string{"order_id": orderID})
}

/* ---------- keyboards ---------- */

func deliveryTypeKeyboard(lang utils.Lang) tgbotapi.ReplyKeyboardMarkup {
	btnDelivery := tgbotapi.NewKeyboardButton(texts.Get(lang, texts.DeliveryBtn))
	btnPickup := tgbotapi.NewKeyboardButton(texts.Get(lang, texts.PickupBtn))
	btnBack := tgbotapi.NewKeyboardButton(texts.Get(lang, texts.BackButton))

	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(btnDelivery, btnPickup),
		tgbotapi.NewKeyboardButtonRow(btnBack),
	)
	kb.ResizeKeyboard = true
	kb.OneTimeKeyboard = true
	return kb
}

func locationKeyboard(lang utils.Lang) tgbotapi.ReplyKeyboardMarkup {
	btnLoc := tgbotapi.NewKeyboardButtonLocation(texts.Get(lang, texts.SendLocationBtn))
	btnBack := tgbotapi.NewKeyboardButton(texts.Get(lang, texts.BackButton))

	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(btnLoc),
		tgbotapi.NewKeyboardButtonRow(btnBack),
	)
	kb.ResizeKeyboard = true
	kb.OneTimeKeyboard = false
	return kb
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

func paymentMethodKeyboard(lang utils.Lang) tgbotapi.ReplyKeyboardMarkup {
	btnPayme := tgbotapi.NewKeyboardButton("💳 Payme")
	btnClick := tgbotapi.NewKeyboardButton("💳 Click")
	btnCash := tgbotapi.NewKeyboardButton("💵 Naqt")
	btnBack := tgbotapi.NewKeyboardButton(texts.Get(lang, texts.BackButton))

	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(btnPayme, btnClick),
		tgbotapi.NewKeyboardButtonRow(btnCash),
		tgbotapi.NewKeyboardButtonRow(btnBack),
	)
	kb.ResizeKeyboard = true
	kb.OneTimeKeyboard = true
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

func toOrderProducts(items []structs.ProductCart) []structs.OrderProduct {
	out := make([]structs.OrderProduct, 0, len(items))
	for _, it := range items {
		out = append(out, structs.OrderProduct{
			ID:           it.Id,
			Quantity:     it.Count,
			ProductPrice: it.Price,
			ProductName:  it.Name,
		})
	}
	return out
}

func (c *Commands) WaitingPaymentHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}

	chatID := ctx.Update().FromChat().ID

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}
	lang := account.Language

	txt := strings.TrimSpace(ctx.Update().Message.Text)

	_, st, _ := ctx.GetState()
	if st == nil {
		st = map[string]string{}
	}

	// Back -> checkout_preview
	if txt == texts.Get(lang, texts.BackButton) {
		_ = ctx.UpdateState("checkout_preview", st)
		c.ShowCheckoutPreview(ctx)
		return
	}

	orderID := strings.TrimSpace(st["order_id"])
	if orderID == "" {
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	ord, err := c.orderSvc.GetByID(ctx.Context, orderID)
	if err != nil {
		c.logger.Error(ctx.Context, "waiting_payment: GetByID failed", zap.Error(err), zap.String("order_id", orderID))
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	// PAID bo'lsa yakunlaymiz
	if strings.ToUpper(strings.TrimSpace(ord.Order.PaymentStatus)) == "PAID" {
		_ = c.cartSvc.Clear(ctx.Context, account.TgID)

		m := tgbotapi.NewMessage(chatID, "✅ To‘lov qabul qilindi. Buyurtmangiz tayyorlanmoqda.")
		m.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		_, _ = ctx.Bot().Send(m)

		_ = ctx.UpdateState("show_main_menu", nil)
		return
	}

	// Aks holda linkni qayta ko'rsatamiz
	payURL := strings.TrimSpace(ord.Order.PaymentUrl)
	if payURL == "" {
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("To‘lash", payURL),
		),
	)
	m := tgbotapi.NewMessage(chatID, "⏳ To‘lov hali yakunlanmadi. Quyidagi havola orqali to‘lovni tugating:")
	m.ReplyMarkup = kb
	_, _ = ctx.Bot().Send(m)
}

// paymentDetailsHTML — 2-chi rasmga o‘xshash: URL yashirin, faqat "To‘lov havolasi" clickable
func paymentDetailsHTML(ord structs.GetListPrimaryKeyResponse) string {
	branch := "SUSHITANA"

	pm := strings.ToUpper(strings.TrimSpace(ord.Order.PaymentMethod))
	if pm == "" {
		pm = "PAYME"
	}

	totalStr := formatSom(ord.Order.TotalPrice)
	link := strings.TrimSpace(ord.Order.PaymentUrl)
	linkEsc := html.EscapeString(link)

	return fmt.Sprintf(
		"🛍 <b>Buyurtmangiz tafsilotlari</b>\n\n"+
			"📦 Buyurtma raqami: <b>#%d</b>\n"+
			"🏬 Filial: <b>%s</b>\n"+
			"💰 Umumiy summa: <b>%s</b> so'm\n"+
			"💸 To‘lov turi: <b>%s</b>\n\n"+
			"Buyurtmangizni tasdiqlash uchun quyidagi havola orqali to‘lovni amalga oshiring:\n\n"+
			"🔗 <a href=\"%s\">To‘lov havolasi</a>\n\n"+
			"✅ To‘lov tugagach, buyurtmangizni tayyorlashni boshlaymiz.",
		ord.Order.OrderNumber,
		html.EscapeString(branch),
		html.EscapeString(totalStr),
		html.EscapeString(pm),
		linkEsc,
	)
}

// formatSom: 270000 -> "270,000"
func formatSom(v int64) string {
	s := strconv.FormatInt(v, 10)
	n := len(s)
	if n <= 3 {
		return s
	}

	var b strings.Builder
	b.Grow(n + n/3)

	rem := n % 3
	if rem == 0 {
		rem = 3
	}

	b.WriteString(s[:rem])
	for i := rem; i < n; i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// eqBtn — agar sizda bot paketida allaqachon bo'lsa, shu yerga ko'chirmang.
// Aks holda, shu faylda ham ishlatish uchun qoldiring.
func eqBtn(got, want string) bool {
	return normBtn(got) == normBtn(want)
}

func normBtn(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "\uFE0F", "")

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		// bu yerda unicode import kerak bo'ladi
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r >= 0x0400 && r <= 0x04FF { // кириллица
			b.WriteRune(r)
		}
	}
	return b.String()
}
