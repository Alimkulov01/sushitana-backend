package order

import (
	"errors"
	"net/http"
	"sushitana/internal/order"
	"sushitana/internal/responses"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	"sushitana/pkg/reply"
	"sushitana/pkg/utils"

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
		CreateOrder(c *gin.Context)
		GetByTgIdOrder(c *gin.Context)
		GetByIDOrder(c *gin.Context)
		GetListOrder(c *gin.Context)
		DeleteOrder(c *gin.Context)
		UpdateStatusOrder(c *gin.Context)
	}
	Params struct {
		fx.In
		Logger       logger.Logger
		OrderService order.Service
	}

	handler struct {
		logger       logger.Logger
		orderService order.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:       p.Logger,
		orderService: p.OrderService,
	}
}

func (h *handler) CreateOrder(c *gin.Context) {
	var (
		response structs.Response
		request  structs.CreateOrder
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	err = h.orderService.Create(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			response = responses.BadRequest
			return
		}
		h.logger.Error(ctx, " err on h.orderService.Create", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}

func (h *handler) GetByTgIdOrder(c *gin.Context) {
	var (
		response structs.Response
		tg_id    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(tg_id)
	cartData, err := h.orderService.GetByTgId(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.orderService.GetByTgID", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = cartData
}

func (h *handler) GetByIDOrder(c *gin.Context) {
	var (
		response structs.Response
		id       = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	cartData, err := h.orderService.GetByID(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.orderService.GetByTgID", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = cartData
}

func (h *handler) DeleteOrder(c *gin.Context) {

	var (
		response structs.Response
		order_id = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	err := h.orderService.Delete(c, order_id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.orderService.Delete", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}

func (h *handler) GetListOrder(c *gin.Context) {
	var (
		response structs.Response
		filter   structs.GetListOrderRequest
		ctx      = c.Request.Context()

		offset        = c.Query("offset")
		limit         = c.Query("limit")
		status        = c.Query("status_order")
		paymentStatus = c.Query("payment_status")
		deliveryType  = c.Query("delivery_type")
		paymentMethod = c.Query("payment_method")
		createdAt     = c.Query("created_at")
	)

	filter.Limit = int64(utils.StrToInt(limit))
	filter.Offset = int64(utils.StrToInt(offset))
	filter.Status = status
	filter.PaymentStatus = paymentStatus
	filter.DeliveryType = deliveryType
	filter.PaymentMethod = paymentMethod
	filter.CreatedAt = createdAt
	defer reply.Json(c.Writer, http.StatusOK, &response)

	list, err := h.orderService.GetList(c, filter)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.orderService.GetAll", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = list
}

func (h *handler) UpdateStatusOrder(c *gin.Context) {
	var (
		response structs.Response
		request  structs.UpdateStatus
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	err = h.orderService.UpdateStatus(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			response = responses.BadRequest
			return
		}
		h.logger.Error(ctx, " err on h.orderService.Create", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}
