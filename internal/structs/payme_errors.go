package structs

const (
	ErrNotPost        = -32300
	ErrParseJSON      = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrNotEnoughPriv  = -32504
	ErrInternal       = -32400
)

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewRPCError(code int, msg string, data any) RPCError {
	return RPCError{Code: code, Message: msg, Data: data}
}
