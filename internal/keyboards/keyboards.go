package keyboards

import (
	"sushitana/pkg/utils"

	tgbotapi "github.com/ilpy20/telegram-bot-api/v7"
)

var LanguageKeyboard = func(lang utils.Lang) tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🇺🇿 Oʻzbekcha"),
			tgbotapi.NewKeyboardButton("🇷🇺 Русский"),
			tgbotapi.NewKeyboardButton("🇬🇧 English"),
		),
	)

	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = true

	return keyboard
}
