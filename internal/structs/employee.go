package structs

import "time"

type Employee struct {
	Id          int64      `json:"id"`
	Name        string     `json:"name"`
	Surname     string     `json:"surname"`
	Username    string     `json:"username"`
	Password    string     `json:"-"`
	IsActive    bool       `json:"is_active"`
	PhoneNumber string     `json:"phone_number"`
	RoleId      string     `json:"role_id"`
	RoleName    string     `json:"role_name,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

type CreateEmployee struct {
	Name        string `json:"name"`
	Surname     string `json:"surname"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	IsActive    bool   `json:"is_active"`
	PhoneNumber string `json:"phone_number"`
	RoleId      string `json:"role_id"`
}

type PatchEmployee struct {
	Id          int64   `json:"id"`
	Name        *string `json:"name"`
	Surname     *string `json:"surname"`
	Username    *string `json:"username"`
	Password    *string `json:"password"`
	IsActive    *bool   `json:"is_active"`
	PhoneNumber *string `json:"phone_number"`
	RoleId      *string `json:"role_id"`
}

type UpdateEmployee struct {
	Id          int64   `json:"id"`
	Name        *string `json:"name"`
	Surname     *string `json:"surname"`
	Username    *string `json:"username"`
	PhoneNumber *string `json:"phone_number"`
}

type ResetPasswordEmployee struct {
	Id       int64  `json:"id"`
	Password string `json:"password"`
}

type GetListEmployeeRequest struct {
	Offset int64  `json:"offset"`
	Limit  int64  `json:"limit"`
	Search string `json:"search"`
}

type GetListEmployeeResponse struct {
	Count     int64      `json:"count"`
	Employees []Employee `json:"employees"`
}
