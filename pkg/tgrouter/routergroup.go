package tgrouter

import (
	"errors"
	"math"
	"slices"

	"go.uber.org/zap"

	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	"sushitana/pkg/tgrouter/interfaces"
)

const abortIndex int8 = math.MaxInt8 >> 1

func (group *RouterGroup) combineMiddlewares(middlewares ...Middleware) []Middleware {
	finalSize := len(group.middlewares) + len(middlewares)
	assert1(finalSize < int(abortIndex), "too many middlewares")
	mergedMws := make([]Middleware, finalSize)
	copy(mergedMws, middlewares)
	copy(mergedMws[len(middlewares):], group.middlewares)
	return mergedMws
}

func (group *RouterGroup) Use(middleware ...Middleware) {
	group.middlewares = append(group.middlewares, middleware...)
}

func On[F FilterType](group *RouterGroup, filter Filter[F], handler Handler, mws ...Middleware) {
	mws = group.combineMiddlewares(mws...)
	for mw := range slices.Values(mws) {
		handler = mw(handler)
	}

	group.addRoute(newRoute(filter, handler))
}

func (group *RouterGroup) State(c *Ctx) {
	if c == nil || c.update == nil {
		return
	}

	c.Context = group.logger.Context(c.Context)
	group.logger.Info(c.Context, "mwState")

	chat := c.update.FromChat()
	if chat == nil {
		// bu update’da chat yo‘q -> state o‘qimaymiz, lekin c.state nil bo‘lib qolmasin
		group.logger.Warn(c.Context, "mwState: FromChat is nil, skip state middleware")
		c.state = &ctxState{
			stateName: new(string),
			data:      make(map[string]string),
		}
		return
	}

	state, data, err := group.stateDB.Get(
		c.Context,
		int(chat.ID),
		int(chat.ID),
	)
	if err != nil && !errors.Is(err, structs.ErrNotFound) {
		group.logger.Error(c.Context, "failed to get state", zap.Error(err))
		return
	}

	if errors.Is(err, structs.ErrNotFound) {
		group.logger.Info(c.Context, "state not found")
		c.state = &ctxState{
			stateName: new(string),
			data:      make(map[string]string),
		}
		return
	}

	c.SetState(state, data)
}

func (group *RouterGroup) addRoute(route Route) {
	if !group.root {
		group.parent.addRoute(route)
	} else {
		group.routes = append(group.routes, route)
	}
}

type RouterGroup struct {
	parent      *RouterGroup
	routes      []Route
	root        bool
	middlewares []Middleware
	stateDB     interfaces.State
	logger      logger.Logger
}

func (group *RouterGroup) Group() *RouterGroup {
	return &RouterGroup{
		parent:  group,
		root:    false,
		logger:  group.logger,
		stateDB: group.stateDB,
	}
}
