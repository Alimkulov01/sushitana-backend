package structs

type Courier struct {
	Id                   int64  `json:"id"`
	FirstName            string `json:"first_name"`
	LastName             string `json:"last_name"`
	Phone                string `json:"phone"`
	Password             string `json:"password"`
	ConfirmPassword      string `json:"confirm_password"`
	IsActive             bool   `json:"is_active"`
	PasportSerial        string `json:"pasport_serial"`
	TotalOrdersCount     int64  `json:"total_orders_count"`
	CompletedOrdersCount int64  `json:"completed_orders_count"`
	CanceledOrdersCount  int64  `json:"canceled_orders_count"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
}

type CreateCourier struct {
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	Phone           string `json:"phone"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	IsActive        bool   `json:"is_active"`
	PasportSerial   string `json:"pasport_serial"`
}

type PatchCourier struct {
	Id              int64   `json:"id"`
	FirstName       *string `json:"first_name"`
	LastName        *string `json:"last_name"`
	Phone           *string `json:"phone"`
	Password        *string `json:"password"`
	ConfirmPassword *string `json:"confirm_password"`
	IsActive        *bool   `json:"is_active"`
	PasportSerial   *string `json:"pasport_serial"`
}

type GetListCourierRequest struct {
	Offset    int64  `json:"offset"`
	Limit     int64  `json:"limit"`
	Phone     string `json:"phone"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}
