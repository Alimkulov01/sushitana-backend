package product

import (
	"errors"
	"net/http"
	"os"

	"sushitana/internal/iiko"
	product "sushitana/internal/product"
	"sushitana/internal/responses"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	"sushitana/pkg/reply"
	"sushitana/pkg/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Handler interface {
		SyncProduct(c *gin.Context)
		GetListProduct(c *gin.Context)
		GetByIDProduct(c *gin.Context)
		DeleteProduct(c *gin.Context)
		PatchProduct(c *gin.Context)
		GetBox(c *gin.Context)
	}
	Params struct {
		fx.In
		Logger         logger.Logger
		ProductService product.Service
		IIKOService    iiko.Service
	}

	handler struct {
		logger         logger.Logger
		productService product.Service
		iikoService    iiko.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:         p.Logger,
		productService: p.ProductService,
		iikoService:    p.IIKOService,
	}
}

func (h *handler) SyncProduct(c *gin.Context) {
	var (
		response structs.Response
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)
	organizationId := os.Getenv("IIKO_ORGANIZATION_ID")
	if organizationId == "" {
		h.logger.Error(ctx, "IIKO_ORGANIZATION_ID not set")
		response = responses.InternalErr
		return
	}

	token, err := h.iikoService.EnsureValidIikoToken(ctx)
	if err != nil {
		h.logger.Error(ctx, "err EnsureValidIikoToken", zap.Error(err))
		response = responses.InternalErr
		return
	}

	resp, err := h.iikoService.GetProduct(ctx, token, structs.GetCategoryMenuRequest{
		OrganizationId: organizationId,
	})
	if err != nil {
		h.logger.Error(ctx, "err on h.iikoService.GetCategory", zap.Error(err))
		response = responses.InternalErr
		return
	}

	err = h.productService.Create(c, resp.Products)
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
}

func (h *handler) GetByIDProduct(c *gin.Context) {
	var (
		response structs.Response
		id       = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
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
		search = c.Query("search")
	)

	filter.Limit = int64(utils.StrToInt(limit))
	filter.Offset = int64(utils.StrToInt(offset))
	filter.Search = search

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
func (h *handler) GetBox(c *gin.Context) {
	var (
		response structs.Response
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	list, err := h.productService.GetBox(c)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.productService.GetBox", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = list
}

func (h *handler) DeleteProduct(c *gin.Context) {

	var (
		response structs.Response
		id       = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
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
