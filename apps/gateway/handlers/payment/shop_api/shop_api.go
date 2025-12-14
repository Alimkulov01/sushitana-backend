package shopapi

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sushitana/internal/payment/usecase"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"go.uber.org/fx"
)

var Module = fx.Provide(New)

type (
	Params struct {
		fx.In
		Logger  logger.Logger
		ShopSvc usecase.Usecase
	}
	Handler interface {
		Prepare(c *gin.Context)
		Complete(c *gin.Context)
	}

	handler struct {
		logger    logger.Logger
		secretKey string
		shopSvc   usecase.Usecase
	}
)

func New(p Params) Handler {
	secret := strings.TrimSpace(os.Getenv("CLICK_SECRET_KEY"))
	return &handler{
		logger:    p.Logger,
		secretKey: secret,
		shopSvc:   p.ShopSvc,
	}
}

func (h *handler) Prepare(c *gin.Context) {
	var req structs.PrepareRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusOK, structs.PrepareResponse{
			Error:     structs.ErrClickRequestError,
			ErrorNote: "Bad request",
		})
		return
	}

	if req.Action != 0 {
		c.JSON(http.StatusOK, structs.PrepareResponse{
			ClickTransID:    req.ClickTransID,
			MerchantTransID: req.MerchantTransID,
			Error:           structs.ErrActionNotFound,
			ErrorNote:       "Action not found",
		})
		return
	}

	if h.secretKey == "" || !h.verifyPrepare(req) {
		c.JSON(http.StatusOK, structs.PrepareResponse{
			ClickTransID:    req.ClickTransID,
			MerchantTransID: req.MerchantTransID,
			Error:           structs.ErrSignCheckFailed,
			ErrorNote:       "SIGN CHECK FAILED!",
		})
		return
	}

	if req.Error != 0 {
		c.JSON(http.StatusOK, structs.PrepareResponse{
			ClickTransID:    req.ClickTransID,
			MerchantTransID: req.MerchantTransID,
			Error:           structs.ErrCancelled,
			ErrorNote:       "Transaction cancelled",
		})
		return
	}

	prepareID, errCode, errNote := h.shopSvc.Prepare(c.Request.Context(), req)

	resp := structs.PrepareResponse{
		ClickTransID:      req.ClickTransID,
		MerchantTransID:   req.MerchantTransID,
		MerchantPrepareID: cast.ToInt64(prepareID),
		Error:             errCode,
		ErrorNote:         errNote,
	}
	c.JSON(http.StatusOK, resp)
}

func (h *handler) Complete(c *gin.Context) {
	var req structs.CompleteRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusOK, structs.CompleteResponse{
			Error:     structs.ErrClickRequestError,
			ErrorNote: "Bad request",
		})
		return
	}

	if req.Action != 1 {
		c.JSON(http.StatusOK, structs.CompleteResponse{
			ClickTransID:    req.ClickTransID,
			MerchantTransID: req.MerchantTransID,
			Error:           structs.ErrActionNotFound,
			ErrorNote:       "Action not found",
		})
		return
	}

	if h.secretKey == "" || !h.verifyComplete(req) {
		c.JSON(http.StatusOK, structs.CompleteResponse{
			ClickTransID:    req.ClickTransID,
			MerchantTransID: req.MerchantTransID,
			Error:           structs.ErrSignCheckFailed,
			ErrorNote:       "SIGN CHECK FAILED!",
		})
		return
	}

	if req.Error != 0 {
		_, _ = h.shopSvc.Cancel(c.Request.Context(), req) // cleanup/reserve release
		c.JSON(http.StatusOK, structs.CompleteResponse{
			ClickTransID:    req.ClickTransID,
			MerchantTransID: req.MerchantTransID,
			Error:           structs.ErrCancelled,
			ErrorNote:       "Transaction cancelled",
		})
		return
	}

	confirmID, errCode, errNote := h.shopSvc.Complete(c.Request.Context(), req)

	resp := structs.CompleteResponse{
		ClickTransID:      req.ClickTransID,
		MerchantTransID:   req.MerchantTransID,
		MerchantConfirmID: confirmID,
		Error:             errCode,
		ErrorNote:         errNote,
	}
	c.JSON(http.StatusOK, resp)
}

func (h *handler) verifyPrepare(r structs.PrepareRequest) bool {
	src := fmt.Sprintf("%d%d%s%s%s%d%s",
		r.ClickTransID,
		r.ServiceID,
		h.secretKey,
		r.MerchantTransID,
		r.Amount,
		r.Action,
		r.SignTime,
	)
	return strings.EqualFold(md5Hex(src), r.SignString)
}

func (h *handler) verifyComplete(r structs.CompleteRequest) bool {
	src := fmt.Sprintf("%d%d%s%s%d%s%d%s",
		r.ClickTransID,
		r.ServiceID,
		h.secretKey,
		r.MerchantTransID,
		r.MerchantPrepareID,
		r.Amount,
		r.Action,
		r.SignTime,
	)
	return strings.EqualFold(md5Hex(src), r.SignString)
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
