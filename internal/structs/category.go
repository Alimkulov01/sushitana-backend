package structs

import "time"

type Category struct {
	ID        int64     `json:"id"`
	Name      Name      `json:"name"`
	PostID    string    `json:"post_id"`
	Index     int64     `json:"index"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Name struct {
	Uz string `json:"uz"`
	Ru string `json:"ru"`
	En string `json:"en"`
}

type CreateCategory struct {
	Name     Name   `json:"name"`
	PostID   string `json:"post_id"`
	Index    int64  `json:"index"`
	IsActive bool   `json:"is_active"`
}

type PatchCategory struct {
	ID       int64   `json:"id"`
	Name     *Name   `json:"name"`
	PostID   *string `json:"post_id"`
	Index    *int64  `json:"index"`
	IsActive *bool   `json:"is_active"`
}

type GetListCategoryRequest struct {
	Offset int64  `json:"offset"`
	Limit  int64  `json:"limit"`
	Search string `json:"search"`
}

type GetListCategoryResponse struct {
	Count      int64      `json:"count"`
	Categories []Category `json:"categories"`
}
