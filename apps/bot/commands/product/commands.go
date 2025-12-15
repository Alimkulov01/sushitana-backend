package product

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"sushitana/internal/cart"
	"sushitana/internal/category"
	"sushitana/internal/product"
	"sushitana/internal/structs"
	"sushitana/internal/texts"
	"sushitana/pkg/logger"
	"sushitana/pkg/tgrouter"
	"sushitana/pkg/utils"
	"sushitana/pkg/utils/ctxman"

	tgbotapi "github.com/ilpy20/telegram-bot-api/v7"
	"github.com/spf13/cast"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Provide(New)

type Params struct {
	fx.In
	Logger      logger.Logger
	ProductSvc  product.Service
	CategorySvc category.Service
	CartSvc     cart.Service
}

type Commands struct {
	logger      logger.Logger
	ProductSvc  product.Service
	CategorySvc category.Service
	CartSvc     cart.Service
}

func New(p Params) Commands {
	return Commands{
		logger:      p.Logger,
		ProductSvc:  p.ProductSvc,
		CategorySvc: p.CategorySvc,
		CartSvc:     p.CartSvc,
	}
}

func getProductNameByLang(lang utils.Lang, name structs.Name) string {
	switch lang {
	case utils.UZ:
		return name.Uz
	case utils.RU:
		return name.Ru
	default:
		return name.En
	}
}

func getProductDescriptionByLang(lang utils.Lang, name structs.Description) string {
	switch lang {
	case utils.UZ:
		return name.Uz
	case utils.RU:
		return name.Ru
	default:
		return name.En
	}
}

func getCategoryNameByLang(lang utils.Lang, name structs.Name) string {
	switch lang {
	case utils.UZ:
		return name.Uz
	case utils.RU:
		return name.Ru
	default:
		return name.En
	}
}

func (c *Commands) ShowMainMenu(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language
	cartTotal := c.getCartTotalCount(ctx, account.TgID)

	rows := [][]tgbotapi.KeyboardButton{
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.MenuButton)),
		),
	}

	if cartTotal > 0 {
		btnText := texts.Get(lang, texts.Cart)
		rows = append(rows, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(btnText),
		))
	}

	rows = append(rows,
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.FeedbackButton)),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.InfoButton)),
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.ContactButton)),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.LanguageButton)),
		),
	)
	keyboard := tgbotapi.NewReplyKeyboard(rows...)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false

	_ = ctx.UpdateState("show_main_menu", map[string]string{
		"last_action": "show_main_menu",
	})

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Welcome))
	msg.ReplyMarkup = keyboard
	_, _ = ctx.Bot().Send(msg)

	menuUrl := "https://sushitana.uz/uz/bot/home"

	btn := tgbotapi.NewInlineKeyboardButtonWebApp(
		texts.Get(lang, texts.MenuButtonWebAppUrl),
		tgbotapi.WebAppInfo{URL: menuUrl},
	)

	msgUrl := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.MenuButtonWebAppInfo))
	msgUrl.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(btn),
	)

	_, _ = ctx.Bot().Send(msgUrl)
}

func (c *Commands) MenuCategoryHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}

	chatID := ctx.Update().FromChat().ID
	text := strings.TrimSpace(ctx.Update().Message.Text)

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	cats, err := c.CategorySvc.GetList(ctx.Context, structs.GetListCategoryRequest{
		Search: texts.Get(lang, text),
	})
	if err != nil {
		c.logger.Error(ctx.Context, "failed to get categories", zap.Error(err))
		ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	var keyboardRows [][]tgbotapi.KeyboardButton
	var row []tgbotapi.KeyboardButton

	for _, cat := range cats.Categories {
		name := getCategoryNameByLang(lang, cat.Name)
		btn := tgbotapi.NewKeyboardButton(name)
		row = append(row, btn)

		if len(row) == 2 {
			keyboardRows = append(keyboardRows, row)
			row = []tgbotapi.KeyboardButton{}
		}
	}
	if len(row) > 0 {
		keyboardRows = append(keyboardRows, row)
	}

	backText := texts.Get(lang, texts.BackButton)
	backRow := tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(backText),
	)
	keyboardRows = append(keyboardRows, backRow)

	keyboard := tgbotapi.NewReplyKeyboard(keyboardRows...)

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectFromMenu))
	msg.ReplyMarkup = keyboard
	ctx.Bot().Send(msg)

	_ = ctx.UpdateState("category_selected", map[string]string{
		"last_action": "show_category",
	})
}

func (c *Commands) CategoryByProductMenu(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID
	text := strings.TrimSpace(ctx.Update().Message.Text)

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	if text == texts.Get(lang, texts.BackButton) {
		c.logger.Info(ctx.Context, "back to main menu")
		c.ShowMainMenu(ctx)
		return
	}

	products, err := c.ProductSvc.GetListCategoryName(ctx.Context, text)
	if err != nil {
		c.logger.Error(ctx.Context, "failed to get products", zap.Error(err))
		ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	var keyboardRows [][]tgbotapi.KeyboardButton
	var row []tgbotapi.KeyboardButton

	for _, prod := range products {
		name := getProductNameByLang(lang, prod.Name)
		btn := tgbotapi.NewKeyboardButton(name)
		row = append(row, btn)
		if len(row) == 2 {
			keyboardRows = append(keyboardRows, row)
			row = []tgbotapi.KeyboardButton{}
		}
	}
	if len(row) > 0 {
		keyboardRows = append(keyboardRows, row)
	}

	backText := texts.Get(lang, texts.BackButton)
	backRow := tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(backText),
	)
	keyboardRows = append(keyboardRows, backRow)

	keyboard := tgbotapi.NewReplyKeyboard(keyboardRows...)

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectFromMenu))
	msg.ReplyMarkup = keyboard

	ctx.Bot().Send(msg)
	_ = ctx.UpdateState("product_selected", map[string]string{
		"last_action":   "show_products",
		"category_name": text,
	})
}

func (c *Commands) ProductInfoHandler(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID
	text := strings.TrimSpace(ctx.Update().Message.Text)

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language
	if text == texts.Get(lang, texts.BackButton) {
		c.logger.Info(ctx.Context, "back to category menu")
		_ = ctx.UpdateState("category_selected", map[string]string{
			"last_action": "show_category",
		})
		c.MenuCategoryHandler(ctx)
		return
	}
	c.ProductInfo(ctx)

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectAmount))
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	if _, err := ctx.Bot().Send(msg); err != nil {
		c.logger.Error(ctx.Context, "failed to remove reply keyboard", zap.Error(err))
	}
}

func (c *Commands) ProductInfo(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID

	removeMsg := tgbotapi.NewMessage(chatID, " ")
	removeMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	if _, err := ctx.Bot().Send(removeMsg); err != nil {
		c.logger.Error(ctx.Context, "failed to remove reply keyboard", zap.Error(err))
	}

	text := strings.TrimSpace(ctx.Update().Message.Text)

	account, ok := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if !ok || account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	resp, err := c.ProductSvc.GetByProductName(ctx.Context, text)
	if err != nil {
		c.logger.Error(ctx.Context, "failed to get product by name", zap.Error(err))
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	name := getProductNameByLang(lang, resp.Name)
	description := getProductDescriptionByLang(lang, resp.Description)

	var b strings.Builder
	fmt.Fprintf(&b, "*%s*\n", name)

	if strings.TrimSpace(description) != "" {
		lines := strings.Split(description, "\n")
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			fmt.Fprintf(&b, "* %s\n", l)
		}
		fmt.Fprintln(&b)
	}

	price := utils.FCurrency(resp.SizePrices[0].Price.CurrentPrice)
	fmt.Fprintf(&b, "\n*%s %s*", price, texts.Get(lang, texts.CurrencySymbol))

	caption := b.String()

	imgSource := strings.TrimSpace(resp.ImgUrl)
	var photoMsg tgbotapi.PhotoConfig

	if imgSource != "" {
		photoMsg = tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(imgSource))
	} else {
		localPath := "https://sushitana.s3.us-east-1.amazonaws.com//40446061-66cb-4eb2-871f-c01a3f431789.png"
		photoMsg = tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(localPath))
	}

	photoMsg.Caption = caption
	photoMsg.ParseMode = "Markdown"

	qty := 1

	addText := texts.Get(lang, texts.AddToCart)
	backText := texts.Get(lang, texts.BackButton)

	_, data, _ := ctx.GetState()
	categoryName := data["category_name"]

	if categoryName == "" {
		categoryName = text
	}

	backData := fmt.Sprintf("back_to_menu:%s", categoryName)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➖", fmt.Sprintf("qty_dec:%s|%d", resp.ID, qty)),
			tgbotapi.NewInlineKeyboardButtonData(strconv.Itoa(qty), "noop:"),
			tgbotapi.NewInlineKeyboardButtonData("➕", fmt.Sprintf("qty_inc:%s|%d", resp.ID, qty)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(addText, fmt.Sprintf("add_to_cart:%s|%d", resp.ID, qty)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(backText, backData),
		),
	)

	photoMsg.ReplyMarkup = keyboard

	if _, err := ctx.Bot().Send(photoMsg); err != nil {
		c.logger.Error(ctx.Context, "failed to send product photo", zap.Error(err))
		msg := tgbotapi.NewMessage(chatID, caption)
		msg.ParseMode = "Markdown"
		_, _ = ctx.Bot().Send(msg)
	}
}
func (c *Commands) Callback(ctx *tgrouter.Ctx) {
	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}

	data := ctx.Update().CallbackQuery.Data

	switch {
	case data == "noop":
		_ = c.answerCb(ctx, "")
	case strings.HasPrefix(data, "back_to_menu:"):
		c.CategoryByProductMenuCallback(ctx)
		return
	case strings.HasPrefix(data, "qty_dec:"):
		c.ChangeQtyCallback(ctx, -1)
		return
	case strings.HasPrefix(data, "qty_inc:"):
		c.ChangeQtyCallback(ctx, +1)
		return
	case strings.HasPrefix(data, "add_to_cart:"):
		c.AddToCartCallback(ctx)
		return
	case strings.HasPrefix(data, "open_cart:"):
		c.OpenCartCallback(ctx)
		return
	case strings.HasPrefix(data, "cart_inc:"):
		c.CartQtyChangeCallback(ctx, +1)
		return
	case strings.HasPrefix(data, "cart_dec:"):
		c.CartQtyChangeCallback(ctx, -1)
		return
	case strings.HasPrefix(data, "cart_del:"):
		c.CartDeleteCallback(ctx)
		return
	case strings.HasPrefix(data, "cart_clear:"):
		c.CartClearCallback(ctx)
		return
	case strings.HasPrefix(data, "cart_back:"):
		c.CartBackCallback(ctx)
		return
	default:
		_ = c.answerCb(ctx, "")
	}
}
func (c *Commands) answerCb(ctx *tgrouter.Ctx, text string) error {
	cb := ctx.Update().CallbackQuery
	if cb == nil {
		return nil
	}
	_, err := ctx.Bot().Request(tgbotapi.NewCallback(cb.ID, text))
	return err
}

func (c *Commands) OpenCartCallback(ctx *tgrouter.Ctx) {
	cb := ctx.Update().CallbackQuery
	if cb == nil {
		return
	}
	_ = c.answerCb(ctx, "")
	c.GetCartInfo(ctx) // ✅ yangi cart message yuboradi
}

func (c *Commands) ChangeQtyCallback(ctx *tgrouter.Ctx, delta int) {
	u := ctx.Update()
	cb := u.CallbackQuery
	if cb == nil || cb.Message == nil {
		return
	}

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	productID, qty, ok := parseProductQty(cb.Data, delta)
	if !ok {
		_ = c.answerCb(ctx, "")
		return
	}

	qty += delta
	if qty < 1 {
		qty = 1
	}

	// Back tugmasining eski callback_data sini saqlab qolamiz
	backCbData := findCallbackData(cb.Message, "back_to_menu:")
	if backCbData == "" {
		backCbData = "noop" // fallback
	}

	markup := buildProductInlineKeyboard(lang, productID, qty, backCbData)

	edit := tgbotapi.NewEditMessageReplyMarkup(cb.Message.Chat.ID, cb.Message.MessageID, markup)
	if _, err := ctx.Bot().Request(edit); err != nil {
		c.logger.Error(ctx.Context, "failed to edit reply markup", zap.Error(err))
	}

	_ = c.answerCb(ctx, "")
}

func (c *Commands) AddToCartCallback(ctx *tgrouter.Ctx) {
	u := ctx.Update()
	cb := u.CallbackQuery
	data := cb.Data

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		c.logger.Error(ctx.Context, "invalid back_to_menu callback data", zap.String("data", data))
		return
	}
	name := strings.TrimSpace(parts[1])
	productID, qty, ok := parseAddToCart(cb.Data)
	if !ok {
		_ = c.answerCb(ctx, "")
		return
	}
	err := c.CartSvc.Create(ctx.Context, structs.CreateCart{
		TGID:      account.TgID,
		ProductID: productID,
		Count:     int64(qty),
	})
	if err != nil {
		c.logger.Error(ctx.Context, "failed to add to cart", zap.Error(err))
		_ = c.answerCb(ctx, texts.Get(lang, texts.Retry))
		return
	}

	_ = c.answerCb(ctx, "Savatga qo‘shildi")
	_ = ctx.UpdateState("product_selected", map[string]string{
		"last_action":   "show_products",
		"category_name": name,
	})

	if _, err := ctx.Bot().Request(tgbotapi.NewCallback(cb.ID, "")); err != nil {
		c.logger.Error(ctx.Context, "failed to answer callback", zap.Error(err))
	}
}
func parseAddToCart(data string) (productID string, qty int, ok bool) {
	if !strings.HasPrefix(data, "add_to_cart:") {
		return "", 0, false
	}
	rest := strings.TrimPrefix(data, "add_to_cart:") // <uuid>|<qty>
	parts := strings.SplitN(rest, "|", 2)
	if len(parts) != 2 {
		return "", 0, false
	}
	productID = parts[0]
	n, err := strconv.Atoi(parts[1])
	if err != nil || n < 1 {
		n = 1
	}
	return productID, n, true
}

func parseProductQty(data string, delta int) (productID string, qty int, ok bool) {
	prefix := "qty_dec:"
	if delta > 0 {
		prefix = "qty_inc:"
	}
	if !strings.HasPrefix(data, prefix) {
		return "", 0, false
	}

	rest := strings.TrimPrefix(data, prefix) // <uuid>|<qty>
	parts := strings.SplitN(rest, "|", 2)
	if len(parts) != 2 {
		return "", 0, false
	}

	productID = parts[0]
	n, err := strconv.Atoi(parts[1])
	if err != nil || n < 1 {
		n = 1
	}
	return productID, n, true
}

func findCallbackData(msg *tgbotapi.Message, startsWith string) string {
	if msg == nil || msg.ReplyMarkup == nil {
		return ""
	}
	for _, row := range msg.ReplyMarkup.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != nil && strings.HasPrefix(*btn.CallbackData, startsWith) {
				return *btn.CallbackData
			}
		}
	}
	return ""
}

func (c *Commands) CategoryByProductMenuCallback(ctx *tgrouter.Ctx) {
	u := ctx.Update()
	cb := u.CallbackQuery
	data := cb.Data

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		c.logger.Error(ctx.Context, "invalid back_to_menu callback data", zap.String("data", data))
		return
	}
	name := strings.TrimSpace(parts[1])

	chatID := cb.Message.Chat.ID

	c.logger.Info(ctx.Context, "back to products menu", zap.String("name", name))

	products, err := c.ProductSvc.GetListCategoryName(ctx.Context, name)
	if err != nil {
		c.logger.Error(ctx.Context, "failed to get products", zap.Error(err))
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	del := tgbotapi.NewDeleteMessage(chatID, cb.Message.MessageID)
	if _, err := ctx.Bot().Request(del); err != nil {
		c.logger.Error(ctx.Context, "failed to delete product message", zap.Error(err))
	}

	var keyboardRows [][]tgbotapi.KeyboardButton
	var row []tgbotapi.KeyboardButton

	for _, prod := range products {
		prodName := getProductNameByLang(lang, prod.Name)
		btn := tgbotapi.NewKeyboardButton(prodName)
		row = append(row, btn)
		if len(row) == 2 {
			keyboardRows = append(keyboardRows, row)
			row = []tgbotapi.KeyboardButton{}
		}
	}
	if len(row) > 0 {
		keyboardRows = append(keyboardRows, row)
	}

	backText := texts.Get(lang, texts.BackButton)
	backRow := tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(backText),
	)
	keyboardRows = append(keyboardRows, backRow)

	keyboard := tgbotapi.NewReplyKeyboard(keyboardRows...)

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectFromMenu))
	msg.ReplyMarkup = keyboard

	_, _ = ctx.Bot().Send(msg)

	_ = ctx.UpdateState("product_selected", map[string]string{
		"last_action":   "show_products",
		"category_name": name,
	})

	if _, err := ctx.Bot().Request(tgbotapi.NewCallback(cb.ID, "")); err != nil {
		c.logger.Error(ctx.Context, "failed to answer callback", zap.Error(err))
	}
}

func buildProductInlineKeyboard(lang utils.Lang, productID string, qty int, backCbData string) tgbotapi.InlineKeyboardMarkup {
	decData := fmt.Sprintf("qty_dec:%s|%d", productID, qty)
	incData := fmt.Sprintf("qty_inc:%s|%d", productID, qty)
	addData := fmt.Sprintf("add_to_cart:%s|%d", productID, qty)

	addText := texts.Get(lang, texts.AddToCart)
	backText := texts.Get(lang, texts.BackButton)

	rowQty := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➖", decData),
		tgbotapi.NewInlineKeyboardButtonData(strconv.Itoa(qty), "noop:"),
		tgbotapi.NewInlineKeyboardButtonData("➕", incData),
	)

	rowAdd := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(addText, addData),
	)

	rowBack := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(backText, backCbData),
	)

	return tgbotapi.NewInlineKeyboardMarkup(rowQty, rowAdd, rowBack)
}

func (c *Commands) getCartTotalCount(ctx *tgrouter.Ctx, tgID int64) int64 {

	items, err := c.CartSvc.GetByUserTgID(ctx.Context, tgID) // <-- shu joyni moslang
	if err != nil {
		c.logger.Error(ctx.Context, "failed to get cart list", zap.Error(err))
		return 0
	}

	var total int64
	for _, it := range items.Cart.Products {
		total += it.Count
	}
	return total
}
func (c *Commands) GetCartInfo(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}
	lang := account.Language

	items, err := c.CartSvc.GetByUserTgID(ctx.Context, account.TgID)
	if err != nil {
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	text, inlineKB := c.buildCartView(lang, items)

	// 1) Cart xabari (inline)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = inlineKB
	_, _ = ctx.Bot().Send(msg)

	// 2) Pastdagi ReplyKeyboard (ko‘rinmas xabar bilan)
	kbMsg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectFromMenu)) // yoki "Выберите действие:"
	kbMsg.ReplyMarkup = cartBottomKeyboard(lang)
	_ = ctx.UpdateState("get_cart", map[string]string{"last_action": "get_cart_info"})
	_, _ = ctx.Bot().Send(kbMsg)
}

func (c *Commands) GetCartInfoHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}

	text := strings.TrimSpace(ctx.Update().Message.Text)
	chatID := ctx.Update().Message.Chat.ID

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language
	switch text {
	case texts.Get(lang, texts.BackButton):
		_ = ctx.UpdateState("show_main_menu", nil)
		c.ShowMainMenu(ctx)
	case texts.Get(lang, texts.CartClear):
		_ = ctx.UpdateState("show_main_menu", nil)
		_ = c.tryClearCart(ctx.Context, account.TgID)
		c.ShowMainMenu(ctx)
	case texts.Get(lang, texts.CartConfirm):
		_ = ctx.UpdateState("show_type_order", map[string]string{"last_action": "show_main_menu"})
		c.MenuCategoryHandler(ctx)
	default:
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectFromMenu)))
	}
}

func (c *Commands) buildCartView(lang utils.Lang, items structs.GetCartByTgID) (string, tgbotapi.InlineKeyboardMarkup) {
	cur := texts.Get(lang, texts.CurrencySymbol)

	var b strings.Builder
	b.WriteString(texts.Get(lang, texts.CartInfoMsg))
	b.WriteString("\n🛒 " + texts.Get(lang, texts.Cart) + ":\n\n")

	products := items.Cart.Products
	if len(products) == 0 {
		b.WriteString(texts.Get(lang, texts.CartEmpty))

		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(texts.Get(lang, texts.BackButton), "cart_back:"),
			),
		)
		return b.String(), kb
	}

	var rows [][]tgbotapi.InlineKeyboardButton

	// top: clear
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(texts.Get(lang, texts.CartClear), "cart_clear:"),
	))

	// list + buttons
	for i, it := range products {
		name := getProductNameByLang(lang, it.Name)

		unit := cast.ToInt64(it.Price)  // numeric -> int64
		count := cast.ToInt64(it.Count) // count -> int64
		if count < 1 {
			count = 1
		}
		sum := unit * count

		fmt.Fprintf(&b, "%d. %s\n", i+1, name)
		fmt.Fprintf(&b, "   %d x %s = %s %s\n\n",
			count,
			utils.FCurrency(float64(unit)),
			utils.FCurrency(float64(sum)),
			cur,
		)

		// ❌ delete row
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d. %s ❌", i+1, name), "cart_del:"+it.Id),
		))

		// ➖ qty + ➕
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➖", "cart_dec:"+it.Id),
			tgbotapi.NewInlineKeyboardButtonData(strconv.FormatInt(count, 10), "noop:"),
			tgbotapi.NewInlineKeyboardButtonData("➕", "cart_inc:"+it.Id),
		))
	}

	total := cast.ToInt64(items.Cart.TotalPrice)
	fmt.Fprintf(&b, "🧾 %s: %s %s", texts.Get(lang, texts.CartTotal), utils.FCurrency(float64(total)), cur)

	// bottom: back
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(texts.Get(lang, texts.BackButton), "cart_back:"),
	))

	return b.String(), tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (c *Commands) CartQtyChangeCallback(ctx *tgrouter.Ctx, delta int64) {
	cb := ctx.Update().CallbackQuery
	if cb == nil || cb.Message == nil {
		return
	}

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}

	var productID string
	if delta > 0 {
		productID = strings.TrimPrefix(cb.Data, "cart_inc:")
	} else {
		productID = strings.TrimPrefix(cb.Data, "cart_dec:")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		_ = c.answerCb(ctx, "")
		return
	}

	// ✅ bu yerda sizning Cart service update methodingiz bo‘lishi kerak:
	// - increment/decrement qilish
	// Hozircha skeleton: agar service'da method bo'lmasa, oddiy xabar chiqaradi.
	if err := c.tryChangeCartCount(ctx.Context, account.TgID, productID, delta); err != nil {
		_ = c.answerCb(ctx, "Cart update yo‘q")
		return
	}

	_ = c.answerCb(ctx, "")
	c.refreshCartMessage(ctx, cb.Message.Chat.ID, cb.Message.MessageID, account.TgID, account.Language)
}

func cartBottomKeyboard(lang utils.Lang) tgbotapi.ReplyKeyboardMarkup {
	back := texts.Get(lang, texts.BackButton)

	clear := texts.Get(lang, texts.CartClear)
	confirm := texts.Get(lang, texts.CartConfirm)

	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(back),
			tgbotapi.NewKeyboardButton(clear),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(confirm),
		),
	)
	kb.ResizeKeyboard = true
	kb.OneTimeKeyboard = false
	return kb
}

func (c *Commands) CartDeleteCallback(ctx *tgrouter.Ctx) {
	cb := ctx.Update().CallbackQuery
	if cb == nil || cb.Message == nil {
		return
	}

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}

	productID := strings.TrimSpace(strings.TrimPrefix(cb.Data, "cart_del:"))
	if productID == "" {
		_ = c.answerCb(ctx, "")
		return
	}

	if err := c.tryDeleteCartItem(ctx.Context, account.TgID, productID); err != nil {
		_ = c.answerCb(ctx, "Delete yo‘q")
		return
	}

	_ = c.answerCb(ctx, "")
	c.refreshCartMessage(ctx, cb.Message.Chat.ID, cb.Message.MessageID, account.TgID, account.Language)
}

func (c *Commands) CartClearCallback(ctx *tgrouter.Ctx) {
	cb := ctx.Update().CallbackQuery
	if cb == nil || cb.Message == nil {
		return
	}

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}

	if err := c.tryClearCart(ctx.Context, account.TgID); err != nil {
		_ = c.answerCb(ctx, "Clear yo‘q")
		return
	}

	_ = c.answerCb(ctx, "")
	c.refreshCartMessage(ctx, cb.Message.Chat.ID, cb.Message.MessageID, account.TgID, account.Language)
}

func (c *Commands) CartBackCallback(ctx *tgrouter.Ctx) {
	cb := ctx.Update().CallbackQuery
	if cb == nil || cb.Message == nil {
		return
	}

	_ = c.answerCb(ctx, "")
	// cart message'ni o‘chirib, main menu ko‘rsatamiz
	_, _ = ctx.Bot().Request(tgbotapi.NewDeleteMessage(cb.Message.Chat.ID, cb.Message.MessageID))
	c.ShowMainMenu(ctx)
}

func (c *Commands) refreshCartMessage(ctx *tgrouter.Ctx, chatID int64, msgID int, tgID int64, lang utils.Lang) {
	items, err := c.CartSvc.GetByUserTgID(ctx.Context, tgID)
	if err != nil {
		c.logger.Error(ctx.Context, "failed to get cart list", zap.Error(err))
		return
	}

	text, kb := c.buildCartView(lang, items)

	edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
	edit.ReplyMarkup = &kb
	_, _ = ctx.Bot().Request(edit)
}

func (c *Commands) tryChangeCartCount(ctx context.Context, tgID int64, productID string, delta int64) error {
	// Variant 1 (eng yaxshi): cart service’da shunaqa method bo‘lsin:
	// ChangeCount(ctx, tgID, productID, delta)
	type changer interface {
		ChangeCount(ctx context.Context, tgID int64, productID string, delta int64) error
	}
	if svc, ok := any(c.CartSvc).(changer); ok {
		return svc.ChangeCount(ctx, tgID, productID, delta)
	}

	// Variant 2: inc uchun Create ishlatish mumkin (delta=+1)
	if delta > 0 {
		return c.CartSvc.Create(ctx, structs.CreateCart{
			TGID:      tgID,
			ProductID: productID,
			Count:     1,
		})
	}

	return errors.New("no cart change method")
}

func (c *Commands) tryDeleteCartItem(ctx context.Context, tgID int64, productID string) error {
	type deleter interface {
		DeleteItem(ctx context.Context, tgID int64, productID string) error
	}
	if svc, ok := any(c.CartSvc).(deleter); ok {
		return svc.DeleteItem(ctx, tgID, productID)
	}
	return errors.New("no cart delete method")
}

func (c *Commands) tryClearCart(ctx context.Context, tgID int64) error {
	type clearer interface {
		Clear(ctx context.Context, tgID int64) error
	}
	if svc, ok := any(c.CartSvc).(clearer); ok {
		return svc.Clear(ctx, tgID)
	}
	return errors.New("no cart clear method")
}
