package order

import (
	"errors"
	"net/http"
	"strings"
	"sushitana/internal/order"
	"sushitana/internal/responses"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	"sushitana/pkg/reply"
	"sushitana/pkg/utils"
	"time"

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
		UpdateStatusPayment(c *gin.Context)
		DeliveryMapFound(c *gin.Context)
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

	pay_url, _, err := h.orderService.Create(c, request)
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
	response.Payload = pay_url

}

func (h *handler) DeliveryMapFound(c *gin.Context) {
	var (
		response structs.Response
		request  structs.MapFoundRequest
		ctx      = c.Request.Context()
		status   = http.StatusOK
	)

	defer reply.Json(c.Writer, status, &response)

	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn(ctx, "error parse request", zap.Error(err))
		response = responses.BadRequest
		status = http.StatusBadRequest
		return
	}

	price, available, err := h.orderService.DeliveryMapFound(ctx, request)
	if err != nil {
		if errors.Is(err, structs.ErrOutOfDeliveryZone) {
			response = responses.Success
			response.Payload = structs.MapFoundResponse{
				Price:     0,
				Available: false,
			}
			return
		}

		h.logger.Error(ctx, "err on h.orderService.DeliveryMapFound", zap.Error(err))
		response = responses.InternalErr
		status = http.StatusInternalServerError
		return
	}

	response = responses.Success
	response.Payload = structs.MapFoundResponse{
		Price:     price,
		Available: available,
	}
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

		offset           = c.Query("offset")
		limit            = c.Query("limit")
		status           = c.Query("status_order")
		paymentStatus    = c.Query("payment_status")
		deliveryType     = c.Query("delivery_type")
		paymentMethod    = c.Query("payment_method")
		createdAt        = c.Query("created_at")
		orderNumber      = c.Query("order_number")
		phoneNumber      = c.Query("phone_number")
		createdAtFromStr = c.Query("created_at_from")
		createdAtToStr   = c.Query("created_at_to")
	)

	filter.Limit = int64(utils.StrToInt(limit))
	filter.Offset = int64(utils.StrToInt(offset))
	filter.Status = status
	filter.PaymentStatus = paymentStatus
	filter.DeliveryType = deliveryType
	filter.PaymentMethod = paymentMethod
	filter.CreatedAt = createdAt
	filter.OrderNumber = cast.ToInt64(orderNumber)
	filter.PhoneNumber = phoneNumber
	if strings.TrimSpace(createdAtFromStr) != "" {
		t, err := time.Parse(time.RFC3339Nano, createdAtFromStr)
		if err != nil {
			response = responses.BadRequest
			response.Message = "invalid created_at_from (RFC3339 expected)"
			defer reply.Json(c.Writer, http.StatusBadRequest, &response)
			return
		}
		filter.CreatedAtFrom = &t
	}

	if strings.TrimSpace(createdAtToStr) != "" {
		t, err := time.Parse(time.RFC3339Nano, createdAtToStr)
		if err != nil {
			response = responses.BadRequest
			response.Message = "invalid created_at_to (RFC3339 expected)"
			defer reply.Json(c.Writer, http.StatusBadRequest, &response)
			return
		}
		filter.CreatedAtTo = &t
	}
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
		h.logger.Error(ctx, " err on h.orderService.UpdateStatusOrder", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}

func (h *handler) UpdateStatusPayment(c *gin.Context) {
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

	err = h.orderService.UpdatePaymentStatus(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			response = responses.BadRequest
			return
		}
		h.logger.Error(ctx, " err on h.orderService.UpdatePaymentStatus", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}
