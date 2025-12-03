package iiko

import (
	"net/http"
	"sushitana/internal/iiko"
	"sushitana/internal/responses"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	"sushitana/pkg/reply"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Provide(New)

type (
	Handler interface {
		GetIikoAccessToken(c *gin.Context)
	}

	Params struct {
		fx.In
		Logger      logger.Logger
		IikoService iiko.Service
	}

	handler struct {
		logger      logger.Logger
		iikoService iiko.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:      p.Logger,
		iikoService: p.IikoService,
	}
}

func (h *handler) GetIikoAccessToken(c *gin.Context) {
	var (
		response structs.Response
		req      structs.IikoClientTokenRequest
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&req)
	if err != nil {
		h.logger.Error(ctx, " err parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}
	resp, err := h.iikoService.GetIikoAccessToken(ctx)
	if err != nil {
		h.logger.Error(ctx, " err create click invoice", zap.Error(err))
		response = responses.InternalErr
		return
	}
	response = responses.Success
	response.Payload = resp
}
