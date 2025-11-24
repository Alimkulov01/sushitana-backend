package structs

import "time"

type Product struct {
	ID          int64       `json:"id"`
	Name        Name        `json:"name"`
	CategoryID  int64       `json:"category_id"`
	ImgUrl      string      `json:"img_url"`
	Price       int64       `json:"price"`
	Count       int64       `json:"count"`
	Description Description `json:"description"`
	IsActive    bool        `json:"is_active"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type Description struct {
	Uz string `json:"uz"`
	Ru string `json:"ru"`
	En string `json:"en"`
}

type CreateProduct struct {
	Name        Name        `json:"name"`
	CategoryID  int64       `json:"category_id"`
	ImgUrl      string      `json:"img_url"`
	Price       int64       `json:"price"`
	Count       int64       `json:"count"`
	Description Description `json:"description"`
	IsActive    bool        `json:"is_active"`
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
	ID          int64        `json:"id"`
	Name        *Name        `json:"name"`
	CategoryID  *int64       `json:"category_id"`
	ImgUrl      *string      `json:"img_url"`
	Price       *int64       `json:"price"`
	Count       *int64       `json:"count"`
	Description *Description `json:"description"`
	IsActive    *bool        `json:"is_active"`
}
