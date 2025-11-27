package structs

type Admin struct {
	Id        string `json:"id"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	RoleId    string `json:"role_id"`
	CreatedAt string `json:"created_at"`
}

type AdminRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	RoleId      string `json:"role_id"`
	PhoneNumber string `json:"phone_number"`
	IsActive    bool   `json:"is_active"`
}

type AdminLogin struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

type AdminPrimaryKey struct {
	Id string `json:"id"`
}

type GetMeResponse struct {
	ID          string `json:"id"`
	UserName    string `json:"username"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Phone       string `json:"phone"`
	Role        Role   `json:"role"`
	LastLogin   string `json:"last_login"`
	IsSuperUser bool   `json:"is_superuser"`
}
