package structs

type Menu struct {
	Category Category  `json:"category"`
	Products []Product `json:"products"`
}
