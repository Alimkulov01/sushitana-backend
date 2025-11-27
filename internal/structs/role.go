package structs

type Role struct {
	Id               string        `json:"id"`
	RoleName         string        `json:"role_name"`
	RoleDescription  string        `json:"role_description"`
	AccessScopes     []AccessScope `json:"access_scopes"`
	AccessScopeCount int64         `json:"access_scope_count,omitempty"`
	CreatedAt        string        `json:"created_at"`
	UpdatedAt        string        `json:"updated_at"`
}

type CreateRole struct {
	RoleName        string  `json:"role_name"`
	RoleDescription string  `json:"role_description"`
	AccessScopes    []int64 `json:"access_scopes"`
}

type RolePrimaryKey struct {
	Id string `json:"id"`
}

type UpdateRole struct {
	Id              string  `json:"id"`
	RoleName        string  `json:"role_name"`
	RoleDescription string  `json:"role_description"`
	AccessScopes    []int64 `json:"access_scopes"`
}

type PatchRole struct {
	Id              string   `json:"id"`
	RoleName        *string  `json:"role_name"`
	RoleDescription *string  `json:"role_description"`
	AccessScopes    *[]int64 `json:"access_scopes"`
}

type GetListRoleRequest struct {
	Offset int64  `json:"offset"`
	Limit  int64  `json:"limit"`
	Search string `json:"search"`
}

type GetListRoleResponse struct {
	Count int64  `json:"count"`
	Roles []Role `json:"roles"`
}

type AccessScope struct {
	Id          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
