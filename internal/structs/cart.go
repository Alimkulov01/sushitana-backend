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
	Name        string   `json:"name"`
	PhoneNumber string   `json:"phone_number"`
	Cart        CartInfo `json:"cart"`
	Language    string   `json:"language"`
	Link        string   `json:"link"`
}

type CartInfo struct {
	TotalPrice int64         `json:"total_price"`
	Products   []ProductCart `json:"products"`
}

type ProductCart struct {
	Id     int64  `json:"id"`
	Count  int64  `json:"count"`
	Price  int64  `json:"price"`
	Name   Name   `json:"name"`
	ImgUrl string `json:"img_url"`
}
