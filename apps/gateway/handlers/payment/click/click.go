package click

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

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
		ClickRepo clickrepo.Repo // GetByRequestId, UpdateOnComplete
		OrderRepo orderrepo.Repo // UpdateStatus
	}
	handler struct {
		logger         logger.Logger
		clickRepo      clickrepo.Repo
		orderRepo      orderrepo.Repo
		merchantSecret string
	}
)

func New(p Params) Handler {
	return &handler{
		logger:    p.Logger,
		clickRepo: p.ClickRepo,
		orderRepo: p.OrderRepo,
	}
}

func (h *handler) Prepare(c *gin.Context) {
	ctx := c.Request.Context()
	h.logger.Info(ctx, "click prepare-check hit", zap.String("method", c.Request.Method), zap.Any("query", c.Request.URL.Query()))

	c.JSON(http.StatusOK, gin.H{
		"error_code": 0,
		"error_note": "OK",
	})
}

func (h *handler) Complete(c *gin.Context) {
	ctx := c.Request.Context()

	// 1) Read body (limit 1MB)
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		h.logger.Error(ctx, "click complete: read body failed", zap.Error(err))
		c.String(http.StatusBadRequest, "bad request")
		return
	}
	_ = c.Request.Body.Close()

	h.logger.Info(ctx, "click complete callback received", zap.ByteString("body", body))

	// 2) Parse JSON
	var payload structs.CompleteCallbackPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error(ctx, "click complete: invalid json", zap.Error(err))
		c.String(http.StatusBadRequest, "invalid json")
		return
	}

	// 3) Verify signature
	if ok := verifySignature(payload, h.merchantSecret); !ok {
		h.logger.Error(ctx, "click complete: invalid signature", zap.Any("payload", payload))
		c.String(http.StatusForbidden, "invalid signature")
		return
	}

	// 4) Idempotency: olingan request_id bo'yicha invoiceni tekshirish
	inv, err := h.clickRepo.GetByRequestId(ctx, payload.RequestId)
	if err != nil && !errors.Is(err, structs.ErrNotFound) {
		h.logger.Error(ctx, "clickRepo.GetByRequestId failed", zap.Error(err), zap.String("request_id", payload.RequestId))
		c.String(http.StatusInternalServerError, "internal error")
		return
	}

	// Agar allaqachon PAID bo'lsa idempotent qaytarish
	if inv.ID != "" && strings.EqualFold(inv.Status, "PAID") {
		h.logger.Info(ctx, "invoice already paid (idempotent)", zap.String("request_id", payload.RequestId), zap.Int64("click_trans_id", payload.ClickTransId))
		respondOKGin(c)
		return
	}

	// 5) Status aniqlash
	var newStatus string
	if payload.ErrorCode == 0 {
		newStatus = "PAID"
	} else {
		newStatus = "FAILED"
	}

	// 6) DB update (repo ichida tx qo'llash ma'qul)
	if err := h.clickRepo.UpdateOnComplete(ctx, payload.RequestId, payload.ClickTransId, newStatus); err != nil {
		h.logger.Error(ctx, "clickRepo.UpdateOnComplete failed", zap.Error(err), zap.String("request_id", payload.RequestId))
		c.String(http.StatusInternalServerError, "internal error")
		return
	}

	// 7) Order statusni yangilash (agar invoice bilan bog'langan bo'lsa)
	updatedInv, err := h.clickRepo.GetByRequestId(ctx, payload.RequestId)
	if err != nil {
		h.logger.Error(ctx, "clickRepo.GetByRequestId after update failed", zap.Error(err), zap.String("request_id", payload.RequestId))
		// Click ga 200 qaytarishdan chetlashmasin, faqat loglash
		respondOKGin(c)
		return
	}

	if updatedInv.OrderID != "" {
		orderStatus := "PAID"
		if newStatus != "PAID" {
			orderStatus = "FAILED"
		}
		if err := h.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: updatedInv.OrderID, Status: orderStatus}); err != nil {
			h.logger.Error(ctx, "OrderRepo.UpdateStatus failed", zap.Error(err), zap.String("order_id", updatedInv.OrderID))
			// davom eting — Click ga OK qaytariladi
		}
	}

	h.logger.Info(ctx, "click complete processed", zap.String("request_id", payload.RequestId), zap.Int64("click_trans_id", payload.ClickTransId), zap.String("status", newStatus))
	respondOKGin(c)
}

// respondOKGin — Click kutgan formatda javob beradi (agar Click docs boshqacha deyilsa uni moslang)
func respondOKGin(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"error_code": 0,
		"error_note": "OK",
	})
}

// verifySignature — sign tekshiruvi (misol formula).
// Diqqat: bu umumiy misol — iltimos Click rasmiy dokumentatsiyasidagi aniq concatenation va hashing formulasini tekshiring.
// Agar Click docs boshqacha tartib ko'rsatgan bo'lsa shu funksiyani moslang.
func verifySignature(p structs.CompleteCallbackPayload, merchantSecret string) bool {
	if p.Sign == "" {
		return false
	}

	clickTransStr := strconv.FormatInt(p.ClickTransId, 10)
	amountStr := strconv.FormatInt(p.Amount, 10)
	actionStr := strconv.Itoa(p.Action)

	// Example concatenation: merchant_trans_id + click_trans_id + amount + action + merchant_secret
	// (Ba'zi manbalarda merchant_id ham qo'shiladi — hujjatga qarab o'zgartiring)
	source := p.MerchantTransId + clickTransStr + amountStr + actionStr + merchantSecret

	h := sha1.New()
	_, _ = h.Write([]byte(source))
	calculated := hex.EncodeToString(h.Sum(nil))

	return strings.EqualFold(calculated, p.Sign)
}
