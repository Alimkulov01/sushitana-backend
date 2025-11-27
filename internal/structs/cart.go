package structs

type Cart struct {
	ID int64 `json:"id"`
	TgUserID int64 `json:"tg_user_id"`
	ProductID int64 `json:"product_id"`
	Quantity int64 `json:"quantity"`
}