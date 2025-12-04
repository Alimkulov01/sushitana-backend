package cart

import (
	"errors"
	"net/http"
	"sushitana/internal/cart"
	"sushitana/internal/responses"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	"sushitana/pkg/reply"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Handler interface {
		CreateCart(c *gin.Context)
		ClearCart(c *gin.Context)
		DeleteCart(c *gin.Context)
		PatchCart(c *gin.Context)
		GetByUserTgID(c *gin.Context)
		GetByTgID(c *gin.Context)
	}
	Params struct {
		fx.In
		Logger      logger.Logger
		CartService cart.Service
	}

	handler struct {
		logger      logger.Logger
		cartService cart.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:      p.Logger,
		cartService: p.CartService,
	}
}

func (h *handler) CreateCart(c *gin.Context) {
	var (
		response structs.Response
		request  structs.CreateCart
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	err = h.cartService.Create(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			response = responses.BadRequest
			return
		}
		h.logger.Error(ctx, " err on h.cartService.Create", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}

func (h *handler) ClearCart(c *gin.Context) {

	var (
		response structs.Response
		tg_id    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(tg_id)
	err := h.cartService.Clear(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.cartService.Clear", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}

func (h *handler) DeleteCart(c *gin.Context) {

	var (
		response structs.Response
		req      structs.DeleteCart
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&req)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}
	err = h.cartService.Delete(c, req)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.cartService.Delete", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}

func (h *handler) PatchCart(c *gin.Context) {
	var (
		response structs.Response
		request  structs.PatchCart
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	rowsAffected, err := h.cartService.Patch(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Warn(ctx, " err on h.cartService.Patch", zap.Error(err))
		response = responses.InternalErr
		return
	}
	response = responses.Success
	response.Payload = rowsAffected
}

func (h *handler) GetByUserTgID(c *gin.Context) {
	var (
		response structs.Response
		tg_id    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(tg_id)
	cartData, err := h.cartService.GetByUserTgID(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.cartService.GetByTgID", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = cartData
}

func (h *handler) GetByTgID(c *gin.Context) {
	var (
		response structs.Response
		tg_id    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(tg_id)
	cartData, err := h.cartService.GetByTgID(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.cartService.GetByTgID", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = cartData
}
