package structs

type Cart struct {
	ID        int64 `json:"id"`
	TGID      int64 `json:"tg_id"`
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

type CreateCart struct {
	TGID      int64 `json:"tg_id"`
	ProductID int64 `json:"product_id"`
	Count     int64 `json:"count"`
}

type DeleteCart struct {
	TGID      int64 `json:"tg_id"`
	ProductID int64 `json:"product_id"`
}

type PatchCart struct {
	TGID      *int64 `json:"tg_id"`
	ProductID *int64 `json:"product_id"`
	Count     *int64 `json:"count"`
}

type GetCartByTgID struct {
	TGID        int64    `json:"tg_id"`
	PhoneNumber string   `json:"phone_number"`
	Cart        CartInfo `json:"cart"`
}

type CartInfo struct {
	TotalPrice int64     `json:"total_price"`
	Products   []Product `json:"products"`
}
