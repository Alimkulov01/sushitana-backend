package click

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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

// -------- helpers (Shop-API sign) --------

func md5hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func normalizeAmount(s string) string {
	return strings.TrimSpace(s)
}

// Click docs: merchant_prepare_id is int. Avoid huge values like click_paydoc_id (can be > 2^31-1).
func safePrepareID(clickPaydocID int64) int64 {
	const mod = int64(2_000_000_000) // < int32 max
	v := clickPaydocID % mod
	if v <= 0 {
		v = 1
	}
	return v
}

func validatePrepareSign(req structs.ClickPrepareRequest, secret string) bool {
	if req.Action == nil {
		return false
	}
	raw := fmt.Sprintf("%d%d%s%s%s%d%s",
		req.ClickTransId,
		req.ServiceId,
		secret,
		req.MerchantTransId,
		normalizeAmount(req.Amount),
		*req.Action,
		req.SignTime,
	)
	return strings.EqualFold(md5hex(raw), req.SignString)
}

func validateCompleteSign(req structs.ClickCompleteRequest, secret string) bool {
	if req.Action == nil {
		return false
	}
	raw := fmt.Sprintf("%d%d%s%s%d%s%d%s",
		req.ClickTransId,
		req.ServiceId,
		secret,
		req.MerchantTransId,
		req.MerchantPrepareId,
		normalizeAmount(req.Amount),
		*req.Action,
		req.SignTime,
	)
	return strings.EqualFold(md5hex(raw), req.SignString)
}

// -------- handlers --------

// Merchant-API (SMS invoice) flow’da Click ba’zan merchant_trans_id bo‘sh yuboradi.
// Bunday holatda Shop callback’ni “break” qilmaslik uchun:
// - SIGN tekshiramiz
// - Error=0 qaytaramiz
// - merchant_prepare_id ni safe (int32 range) qilib qaytaramiz
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
		c.JSON(http.StatusOK, structs.ClickPrepareResponse{Error: -8, ErrorNote: "Error in request from click"})
		return
	}

	h.logger.Info(ctx, "click prepare parsed",
		zap.Int64("click_trans_id", req.ClickTransId),
		zap.Int64("service_id", req.ServiceId),
		zap.Int64("click_paydoc_id", req.ClickPaydocId),
		zap.String("merchant_trans_id", req.MerchantTransId),
		zap.String("amount", req.Amount),
		zap.Any("action", req.Action),
		zap.Any("error", req.Error),
	)

	// action must be 0 for Prepare
	if req.Action == nil || *req.Action != 0 {
		c.JSON(http.StatusOK, structs.ClickPrepareResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -3,
			ErrorNote:       "Action not found",
		})
		return
	}

	secret := os.Getenv("CLICK_SECRET_KEY")
	if secret == "" {
		h.logger.Error(ctx, "CLICK_SECRET_KEY is empty")
		c.JSON(http.StatusOK, structs.ClickPrepareResponse{Error: -8, ErrorNote: "Server config error"})
		return
	}

	if !validatePrepareSign(req, secret) {
		c.JSON(http.StatusOK, structs.ClickPrepareResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -1,
			ErrorNote:       "SIGN CHECK FAILED!",
		})
		return
	}

	// Merchant-API (invoice/SMS) mode: merchant_trans_id can be empty -> do not fail Click flow
	if strings.TrimSpace(req.MerchantTransId) == "" {
		mpid := safePrepareID(req.ClickPaydocId)
		c.JSON(http.StatusOK, structs.ClickPrepareResponse{
			ClickTransId:      req.ClickTransId,
			MerchantTransId:   "",
			MerchantPrepareId: mpid,
			Error:             0,
			ErrorNote:         "Success",
		})
		return
	}

	// Shop flow (merchant_trans_id present) -> use existing service logic (repo update, amount checks, etc.)
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

	b, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(b))

	h.logger.Info(ctx, "click complete incoming",
		zap.String("content_type", c.GetHeader("Content-Type")),
		zap.ByteString("raw_body", b),
		zap.Any("query", c.Request.URL.Query()),
	)

	var req structs.ClickCompleteRequest
	if err := c.ShouldBind(&req); err != nil {
		h.logger.Warn(ctx, "click complete bind failed", zap.Error(err))
		c.JSON(http.StatusOK, structs.ClickCompleteResponse{Error: -8, ErrorNote: "Error in request from click"})
		return
	}

	h.logger.Info(ctx, "click complete parsed",
		zap.Int64("click_trans_id", req.ClickTransId),
		zap.Int64("service_id", req.ServiceId),
		zap.Int64("click_paydoc_id", req.ClickPaydocId),
		zap.String("merchant_trans_id", req.MerchantTransId),
		zap.Int64("merchant_prepare_id", req.MerchantPrepareId),
		zap.String("amount", req.Amount),
		zap.Any("action", req.Action),
		zap.Any("error", req.Error),
	)

	// action must be 1 for Complete
	if req.Action == nil || *req.Action != 1 {
		c.JSON(http.StatusOK, structs.ClickCompleteResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -3,
			ErrorNote:       "Action not found",
		})
		return
	}

	secret := os.Getenv("CLICK_SECRET_KEY")
	if secret == "" {
		h.logger.Error(ctx, "CLICK_SECRET_KEY is empty")
		c.JSON(http.StatusOK, structs.ClickCompleteResponse{Error: -8, ErrorNote: "Server config error"})
		return
	}

	if !validateCompleteSign(req, secret) {
		c.JSON(http.StatusOK, structs.ClickCompleteResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -1,
			ErrorNote:       "SIGN CHECK FAILED!",
		})
		return
	}

	// Merchant-API (invoice/SMS) mode: merchant_trans_id can be empty -> do not fail Click flow
	if strings.TrimSpace(req.MerchantTransId) == "" {
		c.JSON(http.StatusOK, structs.ClickCompleteResponse{
			ClickTransId:      req.ClickTransId,
			MerchantTransId:   "",
			MerchantConfirmId: req.MerchantPrepareId,
			Error:             0,
			ErrorNote:         "Success",
		})
		return
	}

	// Shop flow
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

	// Update order status only for shop flow (merchant_trans_id present)
	if resp.Error == 0 {
		inv, e := h.clickRepo.GetByMerchantTransID(ctx, req.MerchantTransId)
		if e == nil && inv.OrderID.Valid {
			orderStatus := "PAID"
			if req.Error != nil && *req.Error != 0 {
				orderStatus = "UNPAID"
			}
			if ue := h.orderRepo.UpdatePaymentStatus(ctx, structs.UpdateStatus{
				OrderId: inv.OrderID.String,
				Status:  orderStatus,
			}); ue != nil {
				h.logger.Error(ctx, "order payment status update failed", zap.Error(ue))
			}
			err = h.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
				OrderId: inv.OrderID.String,
				Status:  "WAITING_OPERATOR",
			})
			if err != nil {
				h.logger.Error(ctx, "order status update failed", zap.Error(err))
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}
