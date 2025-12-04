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
type GetProductResponse struct {
	CorrelationId string        `json:"correlationId"`
	Products      []IIKOProduct `json:"products"`
	Revision      int64         `json:"revision"`
}

type IikoGroup struct {
	Id               string `json:"id"`
	ParentGroup      string `json:"parentGroup"`
	IsIncludedInMenu bool   `json:"isIncludedInMenu"`
	IsGroupModifier  bool   `json:"isGroupModifier"`
	Name             string `json:"name"`
	IsDeleted        bool   `json:"isDeleted"`
}
