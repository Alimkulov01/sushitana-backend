package iiko

import (
	"context"
	"net/http"
	"os"
	"strings"
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

	secret := strings.TrimSpace(c.Param("secret"))
	expected := strings.TrimSpace(os.Getenv("IIKO_WEBHOOK_SECRET"))

	if secret == "" || expected == "" || !strings.EqualFold(secret, expected) {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid secret"})
		return
	}

	var events []structs.IikoWebhookEvent
	if err := c.ShouldBindJSON(&events); err != nil {
		h.logger.Error(ctx, "IIKO webhook bind json failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info(ctx, "IIKO webhook parsed",
		zap.Int("count", len(events)),
		zap.String("remote_ip", c.ClientIP()),
	)

	for _, evt := range events {
		evt := evt // loop capture fix
		h.logger.Info(ctx, "iiko webhook event",
			zap.String("eventType", evt.EventType),
			zap.String("externalNumber", evt.EventInfo.ExternalNumber),
			zap.String("creationStatus", evt.EventInfo.CreationStatus),
			zap.String("currentStatus", evt.EventInfo.ID), // naming sizdagi structga qarab
			zap.String("paymentStatus", evt.EventInfo.PosID),
		)
		switch strings.ToUpper(strings.TrimSpace(evt.EventType)) {
		case "DELIVERYORDERUPDATE":
			go func() {
				if err := h.orderSvc.HandleIikoDeliveryOrderUpdate(context.Background(), evt); err != nil {
					h.logger.Error(ctx, "HandleIikoDeliveryOrderUpdate failed", zap.Error(err))
				}
			}()
		case "DELIVERYORDERERROR":
			go func() {
				if err := h.orderSvc.HandleIikoDeliveryOrderError(context.Background(), evt); err != nil {
					h.logger.Error(ctx, "HandleIikoDeliveryOrderError failed", zap.Error(err))
				}
			}()
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
