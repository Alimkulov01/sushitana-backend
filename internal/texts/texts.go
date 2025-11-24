package texts

import (
	"sushitana/pkg/utils"
)

type TextKey = string

const (
	// Common
	Welcome               TextKey = "welcome"
	MenuButtonWebAppInfo  TextKey = "menu_button_web_app_info"
	MenuButtonWebAppUrl   TextKey = "menu_button_web_app_url"
	Language              TextKey = "language"
	Retry                 TextKey = "retry"
	SuccessChangeLanguage TextKey = "success_change_language"
	MenuButton            TextKey = "menu_button"
	FeedbackButton        TextKey = "feedback_button"
	InfoButton            TextKey = "info_button"
	ContactButton         TextKey = "contact_button"
	LanguageButton        TextKey = "language_button"
	SelectFromMenu        TextKey = "select_from_menu"
	TypeLanguage          TextKey = "type_language"
	Contact               TextKey = "contact"
	BackButton            TextKey = "back_button"
)

var MapText = map[TextKey]utils.Language{
	Language: {
		RU: "Привет! Выберите язык коммуникации",
		UZ: "Assalomu alaykum! Komunikatsiya tilini tanlang",
	},
	Retry: {
		RU: "Что-то пошло не так, попробуйте снова",
		UZ: "Xatolik yuz berdi, iltimos qaytadan urinib ko'ring",
	},
	SuccessChangeLanguage: {
		RU: "✅ Язык успешно изменен",
		UZ: "✅ Til muvaffaqiyatli o'zgartirildi",
	},
	Welcome: {
		UZ: `👋 Sushi Tana botiga xush kelibsiz!

	🍣 Sizni ko'rib turganimizdan xursandmiz! Boshlash uchun quyidagi menyudan birini tanlang:

	🍽 Menyu: Bizning mazali va yangi taomlarimizga buyurtma bering.

	🚀 Interaktiv menyu: Web sahifa ko'rinishidagi menyu orqali buyurtma berish imkonini beradi.

	✍️ Fikr qoldirish: Xizmatlarimiz haqida o'z fikringizni bildiring.

	ℹ️ Ma'lumotlar: Bizning restoran haqida ko'proq bilib oling.

	☎️ Bog'lanish: Savollaringiz bormi? Biz doimo aloqadamiz!

	🌍 Tilni o'zgartirish: O'zingizga qulay tilni tanlang.`,
		RU: `👋 Добро пожаловать в бот Sushi Tana!

	🍣 Мы очень рады вас видеть! Чтобы начать, выберите один из пунктов меню ниже:

	🍽 Меню — Заказывайте наши вкусные и свежие блюда.

	🚀 Интерактивное меню — Удобный заказ через меню в виде веб-страницы.

	✍️ Оставить отзыв — Поделитесь своим мнением о наших услугах.

	ℹ️ Информация — Узнайте больше о нашем ресторане.

	☎️ Связаться с нами — Есть вопросы? Мы всегда на связи!

	🌍 Сменить язык — Выберите удобный для вас язык.`,
	},
	MenuButtonWebAppInfo: {
		UZ: "🛍 Interaktiv menyuni ochish",
		RU: "🛍 Открыть интерактивное меню",
	},
	MenuButtonWebAppUrl: {
		UZ: "🚀 Interaktiv menyu",
		RU: "🚀 Интерактивное меню",
	},
	MenuButton: {
		UZ: "🍽 Mazali menyu",
		RU: "🍽 Вкусное меню",
	},
	FeedbackButton: {
		UZ: "✍️ Fikr-mulohaza qoldirish",
		RU: "✍️ Оставить отзыв",
	},
	InfoButton: {
		UZ: "ℹ️ Maʼlumotlar",
		RU: "ℹ️ Информация",
	},
	ContactButton: {
		UZ: "☎️ Bogʻlanish",
		RU: "☎️ Связаться",
	},
	LanguageButton: {
		UZ: "🌐 Tilni oʻzgartirish",
		RU: "🌐 Сменить язык",
	},
	SelectFromMenu: {
		UZ: "Iltimos, menyudan kerakli bo‘limni tanlang 👇",
		RU: "Пожалуйста, выберите нужный раздел из меню 👇",
	},
	TypeLanguage: {
		UZ: "🇺🇿 Oʻzbekcha",
		RU: "🇷🇺 Русский",
	},
	Contact: {
		UZ: `❓ Savollaringiz bormi? Biz bilan bog'laning: 
+998981406003`,
		RU: `❓ Остались вопросы? Свяжитесь с нами: 
+998981406003`,
	},
	BackButton: {
		UZ: "🔙 Ortga",
		RU: "🔙 Назад",
	},
}

func Get(lang utils.Lang, key TextKey) string {
	return MapText[key].By(lang)
}
