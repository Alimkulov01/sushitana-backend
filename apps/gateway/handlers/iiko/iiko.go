package iiko

import (
	"fmt"
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
	var (
		response structs.Response
		req      structs.IikoWebhookDeliveryOrderUpdate
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	// 1) secret tekshirish (URL ichida)
	secret := c.Param("secret")
	if secret == "" || secret != os.Getenv("IIKO_WEBHOOK_SECRET") {
		response = responses.BadRequest
		return
	}
	fmt.Println("GET IIKO ACCESS TOKEN", secret)
	// 2) JSON parse
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn(ctx, "iiko webhook parse error", zap.Error(err))
		response = responses.BadRequest
		return
	}

	// 3) Bizga kerak bo‘lgan event: DeliveryOrderUpdate
	fmt.Println(req.EventType)
	if strings.ToUpper(strings.TrimSpace(req.EventType)) != "DELIVERYORDERUPDATE" {
		response = responses.Success
		return
	}

	// 4) OrderService ichida status mapping/update qiling
	if err := h.orderSvc.HandleIikoDeliveryOrderUpdate(ctx, req); err != nil {
		h.logger.Error(ctx, "HandleIikoDeliveryOrderUpdate failed", zap.Error(err))
		// 200 qaytarish — iiko retry behavior’ini boshqarish uchun qulay (logda ko‘rasiz)
		response = responses.Success
		return
	}

	response = responses.Success
}
