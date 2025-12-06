package product

import (
	"fmt"
	"strconv"
	"strings"

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
}

type Commands struct {
	logger      logger.Logger
	ProductSvc  product.Service
	CategorySvc category.Service
}

func New(p Params) Commands {
	return Commands{
		logger:      p.Logger,
		ProductSvc:  p.ProductSvc,
		CategorySvc: p.CategorySvc,
	}
}

func (c *Commands) CategoryByMenu(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID
	text := strings.TrimSpace(ctx.Update().Message.Text)
	account := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
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
	_ = ctx.UpdateState("product_selected", map[string]string{"last_action": "show_products"})
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

func (c *Commands) ProductInfoHandler(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID
	text := strings.TrimSpace(ctx.Update().Message.Text)
	account := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language
	if text == texts.Get(lang, texts.BackButton) {
		c.logger.Info(ctx.Context, "back to category menu")
		_ = ctx.UpdateState("category_selected", map[string]string{"last_action": "show_category"})
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
	price := cast.ToString(resp.SizePrices[0].Price.CurrentPrice)
	fmt.Fprintf(&b, "\n*%s *", price)

	caption := b.String()

	imgSource := strings.TrimSpace(resp.ImgUrl)
	var photoMsg tgbotapi.PhotoConfig

	if imgSource != "" {
		photoMsg = tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(imgSource))
	} else {
		localPath := "/mnt/data/d566edd8-f1f8-4dc4-ad18-9d1c15755a13.png"
		photoMsg = tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(localPath))
	}

	photoMsg.Caption = caption
	photoMsg.ParseMode = "Markdown"

	qty := 1

	addText := texts.Get(lang, texts.AddToCart)
	backText := texts.Get(lang, texts.BackButton)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➖", fmt.Sprintf("qty_dec:%s:%d", resp.ID, qty)),
			tgbotapi.NewInlineKeyboardButtonData(strconv.Itoa(qty), "qty_nop"),
			tgbotapi.NewInlineKeyboardButtonData("➕", fmt.Sprintf("qty_inc:%s:%d", resp.ID, qty)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(addText, fmt.Sprintf("add_to_cart:%s:%d", resp.ID, qty)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(backText, "back_to_menu"),
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

func (c *Commands) ShowMainMenu(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID
	account := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.MenuButton)),
		),
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
	_ = ctx.UpdateState("show_main_menu", map[string]string{
		"last_action": "changed_language",
	})

	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Welcome))
	msg.ReplyMarkup = keyboard
	ctx.Bot().Send(msg)

	menuUrl := fmt.Sprintf("https://your-api.example/pay/mock/checkout?token=%d", chatID)
	msgUrl := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.MenuButtonWebAppInfo))
	msgUrl.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL(texts.Get(lang, texts.MenuButtonWebAppUrl), menuUrl),
		),
	)
	ctx.Bot().Send(msgUrl)
}

func (c *Commands) MenuCategoryHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}
	chatID := ctx.Update().FromChat().ID
	text := strings.TrimSpace(ctx.Update().Message.Text)
	account := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
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
	_ = ctx.UpdateState("category_selected", map[string]string{"last_action": "show_category"})
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

func (c *Commands) Callback(ctx *tgrouter.Ctx) {
	account := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}

	data := ctx.Update().CallbackQuery.Data
	fmt.Println(">>>>>>>>>>>>>>>>>>>>>>>>", data)

	switch {
	case data == "back_to_menu":
		c.ProductInfoHandler(ctx)

	}
}
