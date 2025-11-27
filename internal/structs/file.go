package structs

type Image struct {
	Id        int64  `json:"id"`
	ImageType string `json:"image_type"`
	Image     string `json:"image"`
}

type CreateImage struct {
	ImageType string `json:"image_type"`
	Image     string `json:"image"`
}

type ImagePrimaryKey struct {
	Id int64 `json:"id"`
}

type GetImageRequest struct {
	Id        int64  `json:"id"`
	ImageType string `json:"image_type"`
}

type GetImagerespones struct {
	Images []Image `json:"images"`
}
