package product

import (
	"fmt"
	"strings"

	"sushitana/internal/product"
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
	Logger     logger.Logger
	ProductSvc product.Service
}

type Commands struct {
	logger     logger.Logger
	ProductSvc product.Service
}

func New(p Params) Commands {
	return Commands{
		logger:     p.Logger,
		ProductSvc: p.ProductSvc,
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
	_ = ctx.UpdateState("product_selected", map[string]string{"last_action": "show_product"})
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

func (c *Commands) ProductInfo(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID
	text := strings.TrimSpace(ctx.Update().Message.Text)
	account := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language
	resp, err := c.ProductSvc.GetByProductName(ctx.Context, text)
	if err != nil {
		c.logger.Error(ctx.Context, "failed to get product name", zap.Error(err))
		ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}
	name := getProductNameByLang(lang, resp.Name)
	desription := getProductDescriptionByLang(lang, resp.Description)
	caption := fmt.Sprintf("%s\n\n%s\n\n%d", name, desription, resp.Price)
	imgSource := strings.TrimSpace(resp.ImgUrl)
	var photo tgbotapi.Chattable

	if imgSource != "" {
		photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(imgSource))
		photoMsg.Caption = caption
		photoMsg.ParseMode = "Markdown"
		photo = photoMsg
	} else {
		localPath := "/mnt/data/d566edd8-f1f8-4dc4-ad18-9d1c15755a13.png"
		photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(localPath))
		photoMsg.Caption = caption
		photoMsg.ParseMode = "Markdown"
		photo = photoMsg
	}

	if _, err := ctx.Bot().Send(photo); err != nil {
		c.logger.Error(ctx.Context, "failed to send product photo", zap.Error(err))
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, caption))
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
