package structs

type IikoClientTokenRequest struct {
	ApiLogin string `json:"apiLogin"`
}

type IikoClientTokenResponse struct {
	CorrelationId string `json:"correlationId"`
	Token         string `json:"token"`
}

type GetOrganizationResponse struct {
	CorrelationId string         `json:"correlationId"`
	Organizations []Organization `json:"organizations"`
}

type Organization struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	ResponseType string `json:"responseType"`
	Code         string `json:"code"`
}

type GetCategoryMenuRequest struct {
	OrganizationId string `json:"organizationId"`
}

type GetCategoryResponse struct {
	CorrelationId string      `json:"correlationId"`
	Groups        []IikoGroup `json:"groups"`
	Revision      int64       `json:"revision"`
}

type IikoGroup struct {
	Id               string `json:"id"`
	ParentGroup      string `json:"parentGroup"`
	IsIncludedInMenu bool   `json:"isIncludedInMenu"`
	IsGroupModifier  bool   `json:"isGroupModifier"`
	Name             string `json:"name"`
	IsDeleted        bool   `json:"isDeleted"`
}

type IikoProduct struct {
	Id             string          `json:"id"`
	GroupId        string          `json:"groupId"`
	Weight         float64         `json:"weight"`
	Type           string          `json:"type"`
	OrderItemType  string          `json:"orderItemType"`
	MeasureUnit    string          `json:"measureUnit"`
	SizePrices     []IikoSizePrice `json:"sizePrices"`
	ParentGroup    string          `json:"parentGroup"`
	PaymentSubject string          `json:"paymentSubject"`
	Code           string          `json:"code"`
	Name           string          `json:"name"`
}

type IikoSizePrice struct {
	Price []IikoPrice `json:"price"`
}

type IikoPrice struct {
	CurrentPrice     float64 `json:"currentPrice"`
	IsIncludedInMenu bool    `json:"isIncludedInMenu"`
}
