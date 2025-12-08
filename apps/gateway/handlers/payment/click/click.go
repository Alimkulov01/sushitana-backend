package click

import (
	"net/http"
	"sushitana/internal/payment/click"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

var Module = fx.Provide(New)

type (
	Handler interface {
		CreateClickInvoice(c *gin.Context)
	}

	Params struct {
		fx.In
		Logger       logger.Logger
		ClickService click.Service
	}

	handler struct {
		logger       logger.Logger
		clickService click.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:       p.Logger,
		clickService: p.ClickService,
	}
}

func (h *handler) CreateClickInvoice(c *gin.Context) {
	var req structs.CreateInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	resp, err := h.clickService.CreateClickInvoice(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"invoice_id":  resp.InvoiceId,
		"payment_url": resp.PaymentUrl,
	})
}
