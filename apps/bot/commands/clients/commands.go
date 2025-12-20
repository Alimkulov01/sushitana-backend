package clients

import (
	"context"
	"strings"

	"sushitana/apps/bot/commands/category"
	productcmd "sushitana/apps/bot/commands/product"
	"sushitana/internal/cart"
	"sushitana/internal/client"
	"sushitana/internal/keyboards"
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
	Logger      logger.Logger
	ClientSvc   client.Service
	CategoryCmd category.Commands
	ProductCmd  productcmd.Commands
	CartSvc     cart.Service
}

type Commands struct {
	logger      logger.Logger
	ClientSvc   client.Service
	CategoryCmd category.Commands
	ProductCmd  productcmd.Commands
	CartSvc     cart.Service
}

func New(p Params) Commands {
	return Commands{
		logger:      p.Logger,
		ClientSvc:   p.ClientSvc,
		CategoryCmd: p.CategoryCmd,
		CartSvc:     p.CartSvc,
		ProductCmd:  p.ProductCmd,
	}
}

func (c *Commands) Start(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID
	c.logger.Info(ctx.Context, "start command", zap.Int64("user_tgid", chatID))

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}

	if account.Language == "" {
		c.logger.Info(ctx.Context, "Language empty → asking language...")

		msg := tgbotapi.NewMessage(chatID, texts.Get(utils.UZ, texts.AllLanguageInfo))
		msg.ReplyMarkup = keyboards.LanguageKeyboard(utils.UZ)

		if _, err := ctx.Bot().Send(msg); err != nil {
			c.logger.Error(ctx.Context, "failed to send language keyboard", zap.Error(err))
		}

		_ = ctx.UpdateState("waiting_change_language", map[string]string{"last_action": "edit_language"})
		return
	}

	if account.Name == "" {
		c.logger.Info(ctx.Context, "Name empty → asking name...")
		c.AskName(ctx)
		return
	}

	if account.Phone == "" {
		c.logger.Info(ctx.Context, "Phone empty → requesting phone...")
		c.RequestPhone(ctx)
		return
	}

	_ = ctx.UpdateState("show_main_menu", map[string]string{"last_action": "start_bot"})
	c.ShowMainMenu(ctx)
}

func (c *Commands) AskName(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found in AskName")
		return
	}

	msgText := texts.Get(account.Language, texts.SetNameClient)

	msg := tgbotapi.NewMessage(chatID, msgText)
	if _, err := ctx.Bot().Send(msg); err != nil {
		c.logger.Error(ctx.Context, "failed to send ask name", zap.Error(err))
		return
	}

	_ = ctx.UpdateState("waiting_for_name", map[string]string{"last_action": "ask_name"})
}

func (c *Commands) SaveName(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		c.logger.Info(ctx.Context, "SaveName: empty message")
		return
	}

	chatID := ctx.Update().Message.Chat.ID
	name := strings.TrimSpace(ctx.Update().Message.Text)
	if name == "" {
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Iltimos, ismni matn ko'rinishida yuboring."))
		return
	}

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found in SaveName")
		return
	}

	if err := c.ClientSvc.UpdateName(ctx.Context, account.TgID, name); err != nil {
		c.logger.Error(ctx.Context, "failed to update name", zap.Error(err))
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Xatolik yuz berdi, qayta urinib ko'ring."))
		return
	}

	account.Name = name
	ctx.Context = context.WithValue(ctx.Context, ctxman.AccountKey{}, account)

	if account.Phone == "" {
		c.RequestPhone(ctx)
		return
	}

	_ = ctx.UpdateState("show_main_menu", map[string]string{"last_action": "name_saved"})
	c.ShowMainMenu(ctx)
}

func (c *Commands) RequestPhone(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID

	msg := tgbotapi.NewMessage(chatID, "📞 Telefon raqamingizni yuboring yoki pastdagi tugmani bosing.")

	contactBtn := tgbotapi.NewKeyboardButtonContact("📲 Raqamni yuborish")
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(contactBtn),
	)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = true

	msg.ReplyMarkup = keyboard

	if _, err := ctx.Bot().Send(msg); err != nil {
		c.logger.Error(ctx.Context, "failed to send contact request", zap.Error(err))
		return
	}

	_ = ctx.UpdateState("waiting_for_phone", map[string]string{"last_action": "request_phone"})
}

func (c *Commands) ChangePhone(ctx *tgrouter.Ctx) {
	c.logger.Info(ctx.Context, "ChangePhone handler called", zap.Any("update", ctx.Update()))

	if ctx.Update().Message == nil || ctx.Update().Message.Contact == nil {
		c.logger.Info(ctx.Context, "ChangePhone: no contact in message")
		return
	}

	chatID := ctx.Update().Message.Chat.ID
	contact := ctx.Update().Message.Contact

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}

	if contact.UserID != 0 && contact.UserID != account.TgID {
		c.logger.Warn(ctx.Context, "user sent another person's contact")
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Iltimos, o'zingizning kontaktingizni yuboring."))
		return
	}

	phone := contact.PhoneNumber

	if err := c.ClientSvc.UpdatePhone(ctx.Context, account.TgID, phone); err != nil {
		c.logger.Error(ctx.Context, "failed to update phone", zap.Error(err))
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Xatolik yuz berdi, qayta urinib ko'ring."))
		return
	}

	account.Phone = phone
	ctx.Context = context.WithValue(ctx.Context, ctxman.AccountKey{}, account)

	remove := tgbotapi.NewRemoveKeyboard(true)
	confirm := tgbotapi.NewMessage(chatID, "Raqamingiz saqlandi ✅")
	confirm.ReplyMarkup = remove

	if _, err := ctx.Bot().Send(confirm); err != nil {
		c.logger.Error(ctx.Context, "failed to send phone confirm", zap.Error(err))
	}

	_ = ctx.UpdateState("show_main_menu", map[string]string{"last_action": "phone_saved"})
	c.ShowMainMenu(ctx)
}

func (c *Commands) MainMenuHandler(ctx *tgrouter.Ctx) {
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
	case texts.Get(lang, texts.LanguageButton):
		_ = ctx.UpdateState("change_language", nil)
		c.ChangeLanguageInfo(ctx)
	case texts.Get(lang, texts.ContactButton):
		_ = ctx.UpdateState("contact", nil)
		c.Contact(ctx)
	case texts.Get(lang, texts.MenuButton):
		_ = ctx.UpdateState("show_category", map[string]string{"last_action": "show_main_menu"})
		c.CategoryCmd.MenuCategoryHandler(ctx)
	case texts.Get(lang, texts.Cart):
		_ = ctx.UpdateState("show_cart", map[string]string{"last_action": "show_main_menu"})
		c.ProductCmd.GetCartInfo(ctx)
	default:
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectFromMenu)))
	}
}

func (c *Commands) ChangeLanguageInfo(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Language))
	msg.ReplyMarkup = keyboards.LanguageKeyboard(lang)

	if _, err := ctx.Bot().Send(msg); err != nil {
		c.logger.Error(ctx.Context, "failed to send language keyboard", zap.Error(err))
	}
	_ = ctx.UpdateState("waiting_change_language", map[string]string{"last_action": "change_language"})

}

func (c *Commands) Contact(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
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
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false

	_ = ctx.UpdateState("show_main_menu", map[string]string{
		"last_action": "contact",
	})

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Contact))
	msg.ReplyMarkup = keyboard
	_, _ = ctx.Bot().Send(msg)
}

func (c *Commands) ChangeLanguage(ctx *tgrouter.Ctx) {
	c.logger.Info(ctx.Context, "ChangeLanguage handler called", zap.Any("update", ctx.Update()))

	if ctx.Update().Message == nil {
		c.logger.Info(ctx.Context, "ChangeLanguage: update.Message is nil")
		return
	}

	chatID := ctx.Update().Message.Chat.ID
	text := strings.TrimSpace(ctx.Update().Message.Text)

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}

	switch text {
	case "🇺🇿 Oʻzbekcha":
		account.Language = utils.UZ
	case "🇷🇺 Русский":
		account.Language = utils.RU
	case "🇬🇧 English":
		account.Language = utils.EN
	default:
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(account.Language, texts.SelectFromMenu)))
		return
	}

	if err := c.ClientSvc.UpdateLanguage(ctx.Context, account.TgID, account.Language); err != nil {
		c.logger.Error(ctx.Context, "update language err", zap.Error(err))
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Tilni saqlashda xatolik yuz berdi."))
		return
	}

	ctx.Context = context.WithValue(ctx.Context, ctxman.AccountKey{}, account)

	if account.Name == "" {
		c.AskName(ctx)
		return
	}

	if account.Phone == "" {
		c.RequestPhone(ctx)
		return
	}

	_ = ctx.UpdateState("show_main_menu", map[string]string{"last_action": "language_changed"})
	c.ShowMainMenu(ctx)
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

func (c *Commands) getCartTotalCount(ctx *tgrouter.Ctx, tgID int64) int64 {

	items, err := c.CartSvc.GetByUserTgID(ctx.Context, tgID)
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

func (c *Commands) CartMenuHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}
	chatID := ctx.Update().Message.Chat.ID
	text := strings.TrimSpace(ctx.Update().Message.Text)

	account, _ := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		return
	}
	lang := account.Language

	back := texts.Get(lang, texts.BackButton)
	clear := texts.Get(lang, texts.CartClear)
	confirm := texts.Get(lang, texts.CartConfirm)
	switch text {
	case back:
		_ = ctx.UpdateState("show_main_menu", map[string]string{"last_action": "cart_back"})
		c.ShowMainMenu(ctx)
		return
	case clear:
		_ = c.CartSvc.Clear(ctx.Context, account.TgID)
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "🧹 OK"))
		c.ProductCmd.GetCartInfo(ctx)
		return
	case confirm:
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, confirm))
		return

	default:
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Выберите действие с клавиатуры ниже."))
		return
	}
}
