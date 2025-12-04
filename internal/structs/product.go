package structs

import "time"

type Product struct {
	ID                 string      `json:"id"`
	Name               Name        `json:"name"`
	GroupID            string      `json:"groupId"`
	ProductCategoryID  string      `json:"productCategoryId"`
	Type               string      `json:"type"`
	OrderItemType      string      `json:"orderItemType"`
	MeasureUnit        string      `json:"measureUnit"`
	SizePrices         []SizePrice `json:"sizePrices"`
	DoNotPrintInCheque bool        `json:"doNotPrintInCheque"`
	ParentGroup        string      `json:"parentGroup"`
	Order              int64       `json:"order"`
	PaymentSubject     string      `json:"paymentSubject"`
	Code               string      `json:"code"`
	IsDeleted          bool        `json:"isDeleted"`
	CanSetOpenPrice    bool        `json:"canSetOpenPrice"`
	Splittable         bool        `json:"splittable"`
	Index              int64       `json:"index"`
	IsNew              bool        `json:"isNew"`
	ImgUrl             string      `json:"imgUrl"`
	IsActive           bool        `json:"isActive"`
	IsHaveBox          bool        `json:"isHaveBox"`
	BoxCount           int64       `json:"boxCount"`
	BoxPrice           int64       `json:"boxPrice"`
	Description        Description `json:"description"`
	CreatedAt          time.Time   `json:"createdAt"`
	UpdatedAt          time.Time   `json:"updatedAt"`
	Weight             float64     `json:"weight"`
}

type Description struct {
	Uz string `json:"uz"`
	Ru string `json:"ru"`
	En string `json:"en"`
}

type CreateProduct struct {
	ID                 string      `json:"id"`
	Name               Name        `json:"name"`
	GroupID            string      `json:"group_idd"`
	ProductCategoryID  string      `json:"product_category_id"`
	Type               string      `json:"type"`
	OrderItemType      string      `json:"order_item_type"`
	MeasureUnit        string      `json:"measure_unit"`
	SizePrices         []SizePrice `json:"size_prices"`
	DoNotPrintInCheque bool        `json:"do_not_print_in_cheque"`
	ParentGroup        string      `json:"parent_group"`
	Order              int64       `json:"order"`
	PaymentSubject     string      `json:"payment_subject"`
	Code               string      `json:"code"`
	IsDeleted          bool        `json:"is_deleted"`
	CanSetOpenPrice    bool        `json:"can_set_open_price"`
	Splittable         bool        `json:"splittable"`
	Weight             float64     `json:"weight"`
}

type IIKOProduct struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	GroupID            string      `json:"groupId"`
	ProductCategoryID  string      `json:"productCategoryId"`
	Type               string      `json:"type"`
	OrderItemType      string      `json:"orderItemType"`
	MeasureUnit        string      `json:"measureUnit"`
	SizePrices         []SizePrice `json:"sizePrices"`
	DoNotPrintInCheque bool        `json:"doNotPrintInCheque"`
	ParentGroup        string      `json:"parentGroup"`
	Order              int64       `json:"order"`
	PaymentSubject     string      `json:"paymentSubject"`
	Code               string      `json:"code"`
	IsDeleted          bool        `json:"isDeleted"`
	CanSetOpenPrice    bool        `json:"canSetOpenPrice"`
	Splittable         bool        `json:"splittable"`
	Weight             float64     `json:"weight"`
}

type SizePrice struct {
	SizeID string `json:"sizeId"`
	Price  Price  `json:"price"`
}
type Price struct {
	CurrentPrice       float64 `json:"currentPrice"`
	IsIncludedInMenu   bool    `json:"isIncludedInMenu"`
	NextPrice          float64 `json:"nextPrice"`
	NextIncludedInMenu bool    `json:"nextIncludedInMenu"`
	NextDatePrice      string  `json:"nextDatePrice"`
}

type GetListProductRequest struct {
	Offset int64  `json:"offset"`
	Limit  int64  `json:"limit"`
	Search string `json:"Search"`
}

type GetListProductResponse struct {
	Count    int64     `json:"count"`
	Products []Product `json:"products"`
}

type PatchProduct struct {
	ID          string       `json:"id"`
	Name        *Name        `json:"name"`
	Index       *int64       `json:"index"`
	IsNew       *bool        `json:"isNew"`
	ImgUrl      *string      `json:"imgUrl"`
	IsActive    *bool        `json:"isActive"`
	IsHaveBox   *bool        `json:"isHaveBox"`
	BoxCount    *int64       `json:"boxCount"`
	BoxPrice    *int64       `json:"boxPrice"`
	Description *Description `json:"description"`
}
