package keyboards

import (
	"sushitana/pkg/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var LanguageKeyboard = func(lang utils.Lang) tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🇺🇿 Oʻzbekcha"),
			tgbotapi.NewKeyboardButton("🇷🇺 Русский"),
		),
	)

	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = true
	

	return keyboard
}
