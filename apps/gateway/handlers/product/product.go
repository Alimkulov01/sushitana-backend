package product

import (
	"errors"
	"net/http"

	product "sushitana/internal/product"
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
		CreateProduct(c *gin.Context)
		GetListProduct(c *gin.Context)
		GetByIDProduct(c *gin.Context)
		DeleteProduct(c *gin.Context)
		PatchProduct(c *gin.Context)
	}
	Params struct {
		fx.In
		Logger         logger.Logger
		ProductService product.Service
	}

	handler struct {
		logger         logger.Logger
		productService product.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:         p.Logger,
		productService: p.ProductService,
	}
}

func (h *handler) CreateProduct(c *gin.Context) {
	var (
		response structs.Response
		request  structs.CreateProduct
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	product, err := h.productService.Create(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			response = responses.BadRequest
			return
		}
		h.logger.Error(ctx, " err on h.productService.Create", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = product.ID
}

func (h *handler) GetByIDProduct(c *gin.Context) {
	var (
		response structs.Response
		idStr    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(idStr)
	respond, err := h.productService.GetByID(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.productService.GetByID", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = respond
}

func (h *handler) GetListProduct(c *gin.Context) {
	var (
		response structs.Response
		filter   structs.GetListProductRequest
		ctx      = c.Request.Context()

		offset = c.Query("offset")
		limit  = c.Query("limit")
	)

	filter.Limit = int64(utils.StrToInt(limit))
	filter.Offset = int64(utils.StrToInt(offset))

	defer reply.Json(c.Writer, http.StatusOK, &response)

	list, err := h.productService.GetList(c, filter)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.productService.GetList", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = list
}

func (h *handler) DeleteProduct(c *gin.Context) {

	var (
		response structs.Response
		idStr    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(idStr)
	err := h.productService.Delete(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.productService.Delete", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}

func (h *handler) PatchProduct(c *gin.Context) {
	var (
		response structs.Response
		request  structs.PatchProduct
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	rowsAffected, err := h.productService.Patch(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Warn(ctx, " err on h.productService.Patch", zap.Error(err))
		response = responses.InternalErr
		return
	}
	response = responses.Success
	response.Payload = rowsAffected
}
