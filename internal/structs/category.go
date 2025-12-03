package structs

import "time"

type Category struct {
	ID               string    `json:"id"`
	Name             Name      `json:"name"`
	ParentID         string    `json:"parent_id"`
	IsIncludedInMenu bool      `json:"isIncludedInMenu"`
	IsGroupModifier  bool      `json:"isGroupModifier"`
	PostID           string    `json:"post_id"`
	Index            int64     `json:"index"`
	IsActive         bool      `json:"is_active"`
	IsDeleted        bool      `json:"isDeleted"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Name struct {
	Uz string `json:"uz"`
	Ru string `json:"ru"`
	En string `json:"en"`
}

type CreateCategory struct {
	ID               string `json:"id"`
	ParentID         string `json:"parent_id"`
	IsIncludedInMenu bool   `json:"isIncludedInMenu"`
	IsGroupModifier  bool   `json:"isGroupModifier"`
	Name             Name   `json:"name"`
	IsDeleted        bool   `json:"isDeleted"`
}

type PatchCategory struct {
	ID               string  `json:"id"`
	Name             *Name   `json:"name"`
	PostID           *string `json:"post_id"`
	Index            *int64  `json:"index"`
	IsActive         *bool   `json:"is_active"`
	IsIncludedInMenu *bool   `json:"isIncludedInMenu"`
	IsGroupModifier  *bool   `json:"isGroupModifier"`
	IsDeleted        *bool   `json:"isDeleted"`
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
