package tgrouter

type Route struct {
	filter   Filter[any]
	handlers Handler
	rtype    Type
}

type Type int

const (
	MessageRoute Type = iota + 1
	ConversationRoute
)

type Option func(*Route)

func newRoute[F FilterType](filter Filter[F], handlers Handler, options ...Option) Route {
	r := Route{
		filter:   Filter[any](filter),
		handlers: handlers,
	}

	for _, opt := range options {
		opt(&r)
	}

	switch any(filter).(type) {
	case Filter[StateFilter]:
		r.rtype = ConversationRoute
	}

	return r
}
