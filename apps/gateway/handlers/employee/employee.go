package employee

import (
	"errors"
	"net/http"

	employee "sushitana/internal/employee"
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
		CreateEmployee(c *gin.Context)
		GetListEmployee(c *gin.Context)
		GetByIDEmployee(c *gin.Context)
		DeleteEmployee(c *gin.Context)
		PatchEmployee(c *gin.Context)
	}
	Params struct {
		fx.In
		Logger          logger.Logger
		EmployeeService employee.Service
	}

	handler struct {
		logger          logger.Logger
		employeeService employee.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:          p.Logger,
		employeeService: p.EmployeeService,
	}
}

func (h *handler) CreateEmployee(c *gin.Context) {
	var (
		response structs.Response
		request  structs.CreateEmployee
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	resp, err := h.employeeService.Create(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			response = responses.BadRequest
			return
		}
		h.logger.Error(ctx, " err on h.employeeService.Create", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = resp
}

func (h *handler) GetByIDEmployee(c *gin.Context) {
	var (
		response structs.Response
		idStr    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(idStr)
	respond, err := h.employeeService.GetById(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.employeeService.GetByID", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = respond
}

func (h *handler) GetListEmployee(c *gin.Context) {
	var (
		response structs.Response
		filter   structs.GetListEmployeeRequest
		ctx      = c.Request.Context()

		offset = c.Query("offset")
		limit  = c.Query("limit")
		search = c.Query("search")
	)

	filter.Limit = int64(utils.StrToInt(limit))
	filter.Offset = int64(utils.StrToInt(offset))
	filter.Search = search

	defer reply.Json(c.Writer, http.StatusOK, &response)

	list, err := h.employeeService.GetAll(c, filter)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.employeeService.GetAll", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = list
}

func (h *handler) DeleteEmployee(c *gin.Context) {

	var (
		response structs.Response
		idStr    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(idStr)
	err := h.employeeService.Delete(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.employeeService.Delete", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}

func (h *handler) PatchEmployee(c *gin.Context) {
	var (
		response structs.Response
		request  structs.PatchEmployee
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	rowsAffected, err := h.employeeService.Patch(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Warn(ctx, " err on h.employeeService.Patch", zap.Error(err))
		response = responses.InternalErr
		return
	}
	response = responses.Success
	response.Payload = rowsAffected
}
