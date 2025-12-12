package click

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	clicksvc "sushitana/internal/payment/click"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	orderrepo "sushitana/pkg/repository/postgres/order_repo"
	clickrepo "sushitana/pkg/repository/postgres/payment_repo/click_repo"
)

var Module = fx.Provide(New)

type (
	Handler interface {
		Prepare(c *gin.Context)
		Complete(c *gin.Context)
	}

	Params struct {
		fx.In
		Logger    logger.Logger
		ClickSvc  clicksvc.Service
		ClickRepo clickrepo.Repo
		OrderRepo orderrepo.Repo
	}

	handler struct {
		logger    logger.Logger
		clickSvc  clicksvc.Service
		clickRepo clickrepo.Repo
		orderRepo orderrepo.Repo
	}
)

func New(p Params) Handler {
	return &handler{
		logger:    p.Logger,
		clickSvc:  p.ClickSvc,
		clickRepo: p.ClickRepo,
		orderRepo: p.OrderRepo,
	}
}

func (h *handler) Prepare(c *gin.Context) {
	ctx := c.Request.Context()

	b, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(b))

	h.logger.Info(ctx, "click prepare incoming",
		zap.String("content_type", c.GetHeader("Content-Type")),
		zap.ByteString("raw_body", b),
		zap.Any("query", c.Request.URL.Query()),
	)

	var req structs.ClickPrepareRequest
	if err := c.ShouldBind(&req); err != nil {
		h.logger.Warn(ctx, "click prepare bind failed", zap.Error(err))
		c.JSON(http.StatusOK, structs.ClickPrepareResponse{
			Error:     -8,
			ErrorNote: "Error in request from click",
		})
		return
	}

	resp, err := h.clickSvc.ShopPrepare(ctx, req)
	if err != nil {
		h.logger.Error(ctx, "ShopPrepare failed", zap.Error(err))
		if resp.Error == 0 {
			resp.Error = -8
			resp.ErrorNote = "Server error"
		}
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) Complete(c *gin.Context) {
	ctx := c.Request.Context()

	var req structs.ClickCompleteRequest
	if err := c.ShouldBind(&req); err != nil {
		h.logger.Warn(ctx, "click complete bind failed", zap.Error(err))
		c.JSON(http.StatusOK, structs.ClickCompleteResponse{
			Error:     -8,
			ErrorNote: "Error in request from click",
		})
		return
	}

	resp, err := h.clickSvc.ShopComplete(ctx, req)
	if err != nil {
		h.logger.Error(ctx, "ShopComplete failed", zap.Error(err))
		if resp.Error == 0 {
			resp.Error = -8
			resp.ErrorNote = "Server error"
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	if resp.Error == 0 {
		inv, e := h.clickRepo.GetByMerchantTransID(ctx, req.MerchantTransId)
		if e == nil && inv.OrderID.Valid {
			orderStatus := "PAID"
			if req.Error != 0 {
				orderStatus = "UNPAID"
			}
			if ue := h.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
				OrderId: inv.OrderID.String,
				Status:  orderStatus,
			}); ue != nil {
				h.logger.Error(ctx, "order status update failed", zap.Error(ue))
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}
