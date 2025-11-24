package responses

import "sushitana/internal/structs"

func newResponse(code int, message string) structs.Response {
	return structs.Response{
		Code:    code,
		Message: message}
}

// Response code
const (
	OkCode           = 200
	BadRequestCode   = 400
	UnauthorizedCode = 401
	InternalErrCode  = 500
	NotFoundCode     = 404
	ForbiddenCode    = 403

	BlockedCode = iota + 1500
)

var (
	// success
	Success = newResponse(OkCode, "Успешно выполнено")

	// failure
	BadRequest   = newResponse(BadRequestCode, "Не правильный запрос")
	NotFound     = newResponse(NotFoundCode, "Не найдено")
	Unauthorized = newResponse(UnauthorizedCode, "Unauthorized")
	InternalErr  = newResponse(InternalErrCode, "Внутренняя ошибка сервера")
	UserBlocked  = newResponse(UnauthorizedCode, "UserBlocked")
	Forbidden    = newResponse(ForbiddenCode, "Запрещено")

	Blocked = newResponse(BlockedCode, "Blocked")
)
