package role

import (
	"errors"
	"net/http"

	"sushitana/internal/responses"
	role "sushitana/internal/role"
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
		CreateRole(c *gin.Context)
		GetListRole(c *gin.Context)
		GetByIDRole(c *gin.Context)
		DeleteRole(c *gin.Context)
		PatchRole(c *gin.Context)
	}
	Params struct {
		fx.In
		Logger      logger.Logger
		RoleService role.Service
	}

	handler struct {
		logger      logger.Logger
		roleService role.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:      p.Logger,
		roleService: p.RoleService,
	}
}

func (h *handler) CreateRole(c *gin.Context) {
	var (
		response structs.Response
		request  structs.CreateRole
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	err = h.roleService.Create(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			response = responses.BadRequest
			return
		}
		h.logger.Error(ctx, " err on h.roleService.Create", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = responses.Success
}

func (h *handler) GetByIDRole(c *gin.Context) {
	var (
		response structs.Response
		id       = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	respond, err := h.roleService.GetById(c, structs.RolePrimaryKey{Id: id})
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.roleService.GetByID", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = respond
}

func (h *handler) GetListRole(c *gin.Context) {
	var (
		response structs.Response
		filter   structs.GetListRoleRequest
		ctx      = c.Request.Context()

		offset = c.Query("offset")
		limit  = c.Query("limit")
		search = c.Query("search")
	)

	filter.Limit = int64(utils.StrToInt(limit))
	filter.Offset = int64(utils.StrToInt(offset))
	filter.Search = search

	defer reply.Json(c.Writer, http.StatusOK, &response)

	list, err := h.roleService.GetAll(c, filter)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.roleService.GetAll", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = list
}

func (h *handler) DeleteRole(c *gin.Context) {

	var (
		response structs.Response
		id       = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	err := h.roleService.Delete(c, structs.RolePrimaryKey{Id: id})
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.roleService.Delete", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}

func (h *handler) PatchRole(c *gin.Context) {
	var (
		response structs.Response
		request  structs.PatchRole
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	rowsAffected, err := h.roleService.Patch(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Warn(ctx, " err on h.roleService.Patch", zap.Error(err))
		response = responses.InternalErr
		return
	}
	response = responses.Success
	response.Payload = rowsAffected
}
