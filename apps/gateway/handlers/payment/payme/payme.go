package payme

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sushitana/internal/payment/payme"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

var Module = fx.Provide(New)

type (
	Handler interface {
		Handle(c *gin.Context)
	}

	Params struct {
		fx.In
		Logger  logger.Logger
		PaymeSv payme.Service
	}

	handler struct {
		logger  logger.Logger
		paymeSv payme.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:  p.Logger,
		paymeSv: p.PaymeSv,
	}
}

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      any             `json:"id"`
}

type rpcResp struct {
	JSONRPC string            `json:"jsonrpc"`
	Result  any               `json:"result,omitempty"`
	Error   *structs.RPCError `json:"error,omitempty"`
	ID      any               `json:"id"`
}

func ok(c *gin.Context, id any, result any) {
	c.JSON(http.StatusOK, rpcResp{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	})
}

func fail(c *gin.Context, id any, e structs.RPCError) {
	c.JSON(http.StatusOK, rpcResp{
		JSONRPC: "2.0",
		Error:   &e,
		ID:      id,
	})
}

func notPost(c *gin.Context) {
	fail(c, nil, structs.RPCError{Code: -32300, Message: "Method must be POST"})
}

func unauthorized(c *gin.Context) {
	fail(c, nil, structs.RPCError{Code: -32504, Message: "Insufficient privilege"})
}

func checkPaymeAuth(c *gin.Context) bool {
	key := os.Getenv("PAYME_SECRET_TEST_KEY")
	if key == "" {
		return false
	}

	h := c.GetHeader("Authorization")
	if h == "" || !strings.HasPrefix(h, "Basic ") {
		return false
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(h, "Basic "))
	if err != nil {
		return false
	}

	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 {
		return false
	}

	login := parts[0]
	pass := parts[1]

	if login != "Paycom" {
		return false
	}
	return pass == key
}

func (h *handler) Handle(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		notPost(c)
		return
	}
	if !checkPaymeAuth(c) {
		unauthorized(c)
		return
	}

	var req rpcReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, nil, structs.RPCError{
			Code:    -32700,
			Message: "Parse error",
		})
		return
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		fail(c, req.ID, structs.RPCError{
			Code:    -32600,
			Message: "Invalid Request",
		})
		return
	}

	ctx := c.Request.Context()

	switch req.Method {

	case "CheckPerformTransaction":
		var p structs.PaymeCheckPerformParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			fail(c, req.ID, structs.RPCError{
				Code:    -32600,
				Message: "Invalid params",
			})
			return
		}
		res, e := h.paymeSv.CheckPerformTransaction(ctx, p)
		if e.Code != 0 {
			fail(c, req.ID, e)
			return
		}
		ok(c, req.ID, res)

	case "CreateTransaction":
		var p structs.PaymeCreateParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			fail(c, req.ID, structs.RPCError{
				Code:    -32600,
				Message: "Invalid params",
			})
			return
		}
		res, e := h.paymeSv.CreateTransaction(ctx, p)
		if e.Code != 0 {
			fail(c, req.ID, e)
			return
		}
		ok(c, req.ID, res)

	case "PerformTransaction":
		var p structs.PaymePerformParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			fail(c, req.ID, structs.RPCError{
				Code:    -32600,
				Message: "Invalid params",
			})
			return
		}
		res, e := h.paymeSv.PerformTransaction(ctx, p)
		if e.Code != 0 {
			fail(c, req.ID, e)
			return
		}
		ok(c, req.ID, res)

	case "CancelTransaction":
		var p structs.PaymeCancelParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			fail(c, req.ID, structs.RPCError{
				Code:    -32600,
				Message: "Invalid params",
			})
			return
		}
		res, e := h.paymeSv.CancelTransaction(ctx, p)
		if e.Code != 0 {
			fail(c, req.ID, e)
			return
		}
		ok(c, req.ID, res)

	case "CheckTransaction":
		var p structs.PaymeCheckParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			fail(c, req.ID, structs.RPCError{
				Code:    -32600,
				Message: "Invalid params",
			})
			return
		}
		res, e := h.paymeSv.CheckTransaction(ctx, p)
		if e.Code != 0 {
			fail(c, req.ID, e)
			return
		}
		ok(c, req.ID, res)

	case "GetStatement":
		var p structs.PaymeStatementParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			fail(c, req.ID, structs.RPCError{
				Code:    -32600,
				Message: "Invalid params",
			})
			return
		}
		res, e := h.paymeSv.GetStatement(ctx, p)
		if e.Code != 0 {
			fail(c, req.ID, e)
			return
		}
		ok(c, req.ID, res)

	default:
		fail(c, req.ID, structs.RPCError{
			Code:    -32601,
			Message: "Method not found",
			Data:    req.Method,
		})
	}
}
