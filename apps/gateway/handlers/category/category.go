package category

import (
	"errors"
	"net/http"

	category "sushitana/internal/category"
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
		CreateCategory(c *gin.Context)
		GetListCategory(c *gin.Context)
		GetByIDCategory(c *gin.Context)
		DeleteCategory(c *gin.Context)
		PatchCategory(c *gin.Context)
	}
	Params struct {
		fx.In
		Logger          logger.Logger
		CategoryService category.Service
	}

	handler struct {
		logger          logger.Logger
		categoryService category.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:          p.Logger,
		categoryService: p.CategoryService,
	}
}

func (h *handler) CreateCategory(c *gin.Context) {
	var (
		response structs.Response
		request  structs.CreateCategory
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	category, err := h.categoryService.Create(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			response = responses.BadRequest
			return
		}
		h.logger.Error(ctx, " err on h.categoryService.Create", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = category.ID
}

func (h *handler) GetByIDCategory(c *gin.Context) {
	var (
		response structs.Response
		idStr    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(idStr)
	respond, err := h.categoryService.GetByID(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.categoryService.GetByID", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = respond
}

func (h *handler) GetListCategory(c *gin.Context) {
	var (
		response structs.Response
		filter   structs.GetListCategoryRequest
		ctx      = c.Request.Context()

		offset = c.Query("offset")
		limit  = c.Query("limit")
	)

	filter.Limit = int64(utils.StrToInt(limit))
	filter.Offset = int64(utils.StrToInt(offset))

	defer reply.Json(c.Writer, http.StatusOK, &response)

	list, err := h.categoryService.GetList(c, filter)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.categoryService.GetList", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = list
}

func (h *handler) DeleteCategory(c *gin.Context) {

	var (
		response structs.Response
		idStr    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(idStr)
	err := h.categoryService.Delete(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.categoryService.Delete", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}

func (h *handler) PatchCategory(c *gin.Context) {
	var (
		response structs.Response
		request  structs.PatchCategory
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	rowsAffected, err := h.categoryService.Patch(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Warn(ctx, " err on h.categoryService.Patch", zap.Error(err))
		response = responses.InternalErr
		return
	}
	response = responses.Success
	response.Payload = rowsAffected
}
