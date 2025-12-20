package texts

import (
	"sushitana/pkg/utils"
)

type TextKey = string

const (
	// Common
	Welcome                       TextKey = "welcome"
	MenuButtonWebAppInfo          TextKey = "menu_button_web_app_info"
	MenuButtonWebAppUrl           TextKey = "menu_button_web_app_url"
	AllLanguageInfo               TextKey = "all_language_info"
	Language                      TextKey = "language"
	SetNameClient                 TextKey = "set_name_client"
	Retry                         TextKey = "retry"
	SuccessChangeLanguage         TextKey = "success_change_language"
	MenuButton                    TextKey = "menu_button"
	FeedbackButton                TextKey = "feedback_button"
	InfoButton                    TextKey = "info_button"
	ContactButton                 TextKey = "contact_button"
	LanguageButton                TextKey = "language_button"
	SelectFromMenu                TextKey = "select_from_menu"
	TypeLanguage                  TextKey = "type_language"
	Contact                       TextKey = "contact"
	BackButton                    TextKey = "back_button"
	AddToCart                     TextKey = "add_to_cart"
	SelectAmount                  TextKey = "select_amount"
	CurrencySymbol                TextKey = "currency_symbol"
	AddedToCart                   TextKey = "added_to_cart"
	Cart                          TextKey = "cart"
	CartInfoMsg                   TextKey = "cart_info_msg"
	CartClear                     TextKey = "cart_clear"
	CartTotal                     TextKey = "cart_total"
	CartConfirm                   TextKey = "cart_confirm"
	CartEmpty                     TextKey = "cart_empty"
	CartItemDeleteHint            TextKey = "cart_item_delete_hint"
	CartQtyHint                   TextKey = "cart_qty_hint"
	CartClearHint                 TextKey = "cart_clear_hint"
	DeliveryBtn                   TextKey = "delivery_btn"
	PickupBtn                     TextKey = "pickup_btn"
	SelectDeliveryType            TextKey = "select_delivery_type"
	AskSendLocation               TextKey = "ask_send_location"
	SendLocationBtn               TextKey = "send_location_btn"
	CancelBtn                     TextKey = "cancel_btn"
	OrderPreviewTitle             TextKey = "order_preview_title"
	OrderPreviewName              TextKey = "order_preview_name"
	OrderPreviewPhone             TextKey = "order_preview_phone"
	OrderPreviewTotal             TextKey = "order_preview_total"
	OrderTypeDelivery             TextKey = "order_type_delivery"
	OrderTypePickup               TextKey = "order_type_pickup"
	OrderDeliveryTypeChoose       TextKey = "order_delivery_type_choose"
	DeliveryTypeDelivery          TextKey = "delivery_type_delivery"
	OrderChoosePaymentMethod      TextKey = "order_choose_payment_method"
	OrderFinishPayment            TextKey = "order_finish_payment"
	OrderAcceptedWaitOperator     TextKey = "order_accepted_wait_operator"
	OrderAddressSavedSendLocation TextKey = "order_address_saved_send_location"
	// Order status notify
	OrderNotifyTitle      TextKey = "order_notify_title"
	OrderNotifyStatusLine TextKey = "order_notify_status_line" // format: "Holat: %s"

	// Status labels
	OrderStatusWaitingPayment  TextKey = "order_status_waiting_payment"
	OrderStatusWaitingOperator TextKey = "order_status_waiting_operator"
	OrderStatusCooking         TextKey = "order_status_cooking"
	OrderStatusReadyForPickup  TextKey = "order_status_ready_for_pickup"
	OrderStatusOnTheWay        TextKey = "order_status_on_the_way"
	OrderStatusDelivered       TextKey = "order_status_delivered"
	OrderStatusCompleted       TextKey = "order_status_completed"
	OrderStatusCancelled       TextKey = "order_status_cancelled"
	OrderStatusRejected        TextKey = "order_status_rejected"
)

var MapText = map[TextKey]utils.Language{
	AllLanguageInfo: {
		UZ: "🇺🇿 Iltimos, suhbat uchun qulay tilni tanlang:\n\n🇷🇺 Пожалуйста, выберите удобный язык для общения:\n\n🇬🇧 Please choose a language for the conversation:",
	},
	Language: {
		UZ: "🌍 Tilni o'zgartirish",
		RU: "🌍 Сменить язык",
		EN: "🌍 Change language",
	},
	SetNameClient: {
		UZ: "Iltimos, ismingizni yuboring.",
		RU: "Пожалуйста, отправьте ваше имя.",
		EN: "Please send your name.",
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
		EN: `
			👋 Welcome to the Sushi Tana bot!

	🍣 We’re happy to see you! To get started, please choose one of the menu options below:

	🍽 Menu — Order our delicious and fresh dishes.

	🚀 Interactive Menu — Place your order through our convenient web-style menu.

	✍️ Leave Feedback — Share your thoughts about our service.

	ℹ️ Information — Learn more about our restaurant.

	☎️ Contact Us — Have questions? We’re always here to help!

	🌍 Change Language — Select the language that suits you best.`,
	},
	MenuButtonWebAppInfo: {
		UZ: "🛍 Interaktiv menyuni ochish",
		RU: "🛍 Открыть интерактивное меню",
		EN: "🛍 Open interactive menu",
	},
	MenuButtonWebAppUrl: {
		UZ: "🚀 Interaktiv menyu",
		RU: "🚀 Интерактивное меню",
		EN: "🚀 Interactive menu",
	},
	MenuButton: {
		UZ: "🍽 Mazali menyu",
		RU: "🍽 Вкусное меню",
		EN: "🍽 Delicious menu",
	},
	FeedbackButton: {
		UZ: "✍️ Fikr-mulohaza qoldirish",
		RU: "✍️ Оставить отзыв",
		EN: "✍️ Leave feedback",
	},
	InfoButton: {
		UZ: "ℹ️ Maʼlumotlar",
		RU: "ℹ️ Информация",
		EN: "ℹ️ Information",
	},
	ContactButton: {
		UZ: "☎️ Bogʻlanish",
		RU: "☎️ Связаться",
		EN: "☎️ Contact",
	},
	LanguageButton: {
		UZ: "🌐 Tilni oʻzgartirish",
		RU: "🌐 Сменить язык",
		EN: "🌐 Change language",
	},
	SelectFromMenu: {
		UZ: "Iltimos, menyudan kerakli bo‘limni tanlang 👇",
		RU: "Пожалуйста, выберите нужный раздел из меню 👇",
		EN: "Please choose the desired section from the menu 👇",
	},
	TypeLanguage: {
		UZ: "🇺🇿 Oʻzbekcha",
		RU: "🇷🇺 Русский",
		EN: "🇬🇧 English",
	},
	Contact: {
		UZ: `❓ Savollaringiz bormi? Biz bilan bog'laning: 
+998981406003`,
		RU: `❓ Остались вопросы? Свяжитесь с нами: 
+998981406003`,
		EN: `❓ Have any questions? Contact us:
+998981406003`,
	},
	BackButton: {
		UZ: "🔙 Ortga",
		RU: "🔙 Назад",
		EN: "🔙 Back",
	},
	AddToCart: {
		UZ: "Qo'shish 🛒",
		RU: "Добавить 🛒",
		EN: "Add 🛒",
	},
	SelectAmount: {
		UZ: "Iltimos, miqdorni tanlang:",
		RU: "Пожалуйста, выберите количество:",
		EN: "Please select the quantity:",
	},
	CurrencySymbol: {
		UZ: "So'm",
		RU: "Сум",
		EN: "UZS",
	},
	Cart: {
		UZ: "🛒 Savatcha",
		RU: "🛒 Корзина",
		EN: "🛒 Cart",
	},
	CartInfoMsg: {
		UZ: `❌ Mahsulot nomi - savatdan olib tashlash

➖ va ➕ - miqdorni kamaytirish yoki oshirish

🔄 Savatni tozalash`,
		RU: `❌ Название товара - удалить из корзины

➖ и ➕ уменьшить или увеличить количество товара

🔄 Очистить корзину`,
		EN: `❌ Item name - remove from cart

➖ and ➕ - decrease or increase the quantity of the item

🔄 Clear cart`,
	},
	CartClear: {
		UZ: "🔄 Savatchani tozalash",
		RU: "🔄 Очистить корзину",
		EN: "🔄 Clear cart",
	},
	CartTotal: {
		UZ: "Jami",
		RU: "Итого",
		EN: "Total",
	},
	CartConfirm: {
		UZ: "✅ Tasdiqlash!",
		RU: "✅ Подтвердить!",
		EN: "✅ Confirm!",
	},
	CartEmpty: {
		UZ: "🛒 Savatcha bo‘sh",
		RU: "🛒 Корзина пуста",
		EN: "🛒 Cart is empty",
	},
	AddedToCart: {
		UZ: "Savatga qo‘shildi",
		RU: "Добавлено в корзину",
		EN: "Added to cart",
	},
	CartItemDeleteHint: {
		UZ: "Mahsulot nomi — savatdan o‘chirish",
		RU: "Название товара — удалить из корзины",
		EN: "Product name — remove from cart",
	},
	CartQtyHint: {
		UZ: "➖ va ➕ — miqdorni kamaytirish yoki oshirish",
		RU: "➖ и ➕ — уменьшить или увеличить количество товара",
		EN: "➖ and ➕ — decrease or increase quantity",
	},
	CartClearHint: {
		UZ: "🔄 Savatni tozalash",
		RU: "🔄 Очистить корзину",
		EN: "🔄 Clear cart",
	},
	DeliveryBtn: {
		UZ: "🚚 Yetkazib berish",
		RU: "🚚 Доставка",
		EN: "🚚 Delivery",
	},
	PickupBtn: {
		UZ: "🏃 Olib ketish",
		RU: "🏃 Самовывоз",
		EN: "🏃 Pickup",
	},
	SelectDeliveryType: {
		UZ: "Yetkazib berish turini tanlang",
		RU: "Выберите доставку или самовывоз",
		EN: "Choose delivery or pickup",
	},
	AskSendLocation: {
		UZ: "Lokatsiya yuboring yoki manzilni yozing:",
		RU: "Отправьте локацию или напишите адрес доставки:",
		EN: "Send your location or type the delivery address:",
	},
	SendLocationBtn: {
		UZ: "📍 Lokatsiyani yuborish",
		RU: "📍 Отправить локацию",
		EN: "📍 Send location",
	},
	CancelBtn: {
		UZ: "❌ Bekor qilish",
		RU: "❌ Отменить",
		EN: "❌ Cancel",
	},
	OrderPreviewTitle: {
		UZ: "📁 Sizning buyurtmangiz:\n\n",
		RU: "📁 Ваш заказ:\n\n",
		EN: "📁 Your order:\n\n",
	},
	OrderPreviewName: {
		UZ: "👤 Ism: %s\n",
		RU: "👤 Имя: %s\n",
		EN: "👤 Name: %s\n",
	},
	OrderPreviewPhone: {
		UZ: "📞 Telefon: %s\n",
		RU: "📞 Телефон: %s\n",
		EN: "📞 Phone: %s\n",
	},
	OrderPreviewTotal: {
		UZ: "💰 Jami: %v so'm",
		RU: "💰 Итого: %v сум",
		EN: "💰 Total: %v UZS",
	},
	OrderTypeDelivery: {
		UZ: "🚚 Buyurtma turi: Yetkazib berish",
		RU: "🚚 Тип заказа: Доставка",
		EN: "🚚 Order type: Delivery",
	},
	OrderTypePickup: {
		UZ: "🚶 Buyurtma turi: Olib ketish",
		RU: "🚶 Тип заказа: Самовывоз",
		EN: "🚶 Order type: Pickup",
	},
	OrderDeliveryTypeChoose: {
		UZ: "Yetkazib berish turini tanlang:",
		RU: "Выберите способ получения:",
		EN: "Choose delivery method:",
	},
	OrderChoosePaymentMethod: {
		UZ: "To‘lov turini tanlang:",
		RU: "Выберите способ оплаты:",
		EN: "Choose payment method:",
	},
	OrderFinishPayment: {
		UZ: "To‘lovni yakunlang:",
		RU: "Завершите оплату:",
		EN: "Complete the payment:",
	},
	OrderAcceptedWaitOperator: {
		UZ: "Buyurtma qabul qilindi ✅ Operator tasdiqlashini kuting.",
		RU: "Заказ принят ✅ Ожидайте подтверждения оператора.",
		EN: "Order accepted ✅ Please wait for operator confirmation.",
	},
	OrderAddressSavedSendLocation: {
		UZ: "Manzil saqlandi ✅ Endi lokatsiyani yuboring.",
		RU: "Адрес сохранён ✅ Теперь отправьте геолокацию.",
		EN: "Address saved ✅ Now share your location.",
	},
	OrderNotifyTitle: {
		UZ: "📦 Zakaz holati",
		RU: "📦 Статус заказа",
		EN: "📦 Order status",
	},
	OrderNotifyStatusLine: {
		UZ: "Holat: %s",
		RU: "Статус: %s",
		EN: "Status: %s",
	},

	OrderStatusWaitingPayment: {
		UZ: "💳 To‘lov kutilmoqda",
		RU: "💳 Ожидается оплата",
		EN: "💳 Waiting for payment",
	},
	OrderStatusWaitingOperator: {
		UZ: "☎️ Operator tasdiqlashi kutilmoqda",
		RU: "☎️ Ожидает подтверждения",
		EN: "☎️ Waiting for confirmation",
	},
	OrderStatusCooking: {
		UZ: "👨‍🍳 Tayyorlanyapti",
		RU: "👨‍🍳 Готовится",
		EN: "👨‍🍳 Preparing",
	},
	OrderStatusReadyForPickup: {
		UZ: "📦 Olib ketishga tayyor",
		RU: "📦 Готово к самовывозу",
		EN: "📦 Ready for pickup",
	},
	OrderStatusOnTheWay: {
		UZ: "🛵 Yo‘lda",
		RU: "🛵 В пути",
		EN: "🛵 On the way",
	},
	OrderStatusDelivered: {
		UZ: "✅ Yetkazildi",
		RU: "✅ Доставлено",
		EN: "✅ Delivered",
	},
	OrderStatusCompleted: {
		UZ: "🎉 Yakunlandi",
		RU: "🎉 Завершено",
		EN: "🎉 Completed",
	},
	OrderStatusCancelled: {
		UZ: "❌ Bekor qilindi",
		RU: "❌ Отменено",
		EN: "❌ Cancelled",
	},
	OrderStatusRejected: {
		UZ: "❌ Qabul qilinmadi",
		RU: "❌ Не принято",
		EN: "❌ Rejected",
	},
}

func Get(lang utils.Lang, key TextKey) string {
	return MapText[key].By(lang)
}
