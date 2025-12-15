package product

import (
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
	_ = c.answerCb(ctx, "Savatcha")

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

func (c *Commands) cartMeta(ctx *tgrouter.Ctx, tgID int64, productID string) (total int64, thisCount int64, err error) {

	items, err := c.CartSvc.GetByUserTgID(ctx.Context, tgID)
	if err != nil {
		return 0, 0, err
	}

	total += items.Cart.TotalPrice
	thisCount = cast.ToInt64(len(items.Cart.Products))

	return total, thisCount, nil
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
