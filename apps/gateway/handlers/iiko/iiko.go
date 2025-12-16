package iiko

import (
	"bytes"
	"io"
	"net/http"
	"sushitana/internal/iiko"
	"sushitana/internal/order"
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
		DeliveryOrderUpdate(c *gin.Context)
	}

	Params struct {
		fx.In
		Logger   logger.Logger
		OrderSvc order.Service
		IikoSvc  iiko.Service
	}

	handler struct {
		logger      logger.Logger
		orderSvc    order.Service
		iikoService iiko.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:      p.Logger,
		orderSvc:    p.OrderSvc,
		iikoService: p.IikoSvc,
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

func (h *handler) DeliveryOrderUpdate(c *gin.Context) {
	ctx := c.Request.Context()

	raw, _ := io.ReadAll(c.Request.Body)
	h.logger.Info(ctx, "IIKO webhook incoming",
		zap.String("path", c.FullPath()),
		zap.String("remote_ip", c.ClientIP()),
		zap.Int("body_len", len(raw)),
		zap.ByteString("body", raw),
	)

	// body qayta o‘qilishi uchun
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))

	var evt structs.IikoWebhookDeliveryOrderUpdate
	if err := c.ShouldBindJSON(&evt); err != nil {
		h.logger.Error(ctx, "IIKO webhook bind json failed", zap.Error(err))
		c.JSON(200, gin.H{"ok": true}) // iiko qayta qayta urmasin
		return
	}

	h.logger.Info(ctx, "IIKO webhook parsed",
		zap.String("eventType", evt.EventType),
		zap.String("correlationId", evt.CorrelationID),
		zap.String("externalNumber", evt.EventInfo.ExternalNumber),
		zap.String("creationStatus", evt.EventInfo.CreationStatus),
	)

	_ = h.orderSvc.HandleIikoDeliveryOrderUpdate(ctx, evt)
	c.JSON(200, gin.H{"ok": true})
}
