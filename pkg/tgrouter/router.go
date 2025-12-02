package tgrouter

import (
	"context"
	"slices"
	"sync"
	"time"

	tgbotapi "github.com/ilpy20/telegram-bot-api/v7"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/exp/rand"

	"sushitana/pkg/logger"
	"sushitana/pkg/tgrouter/interfaces"
)

var Module = fx.Provide(NewRouterFactory)

type RouterFactory func(*tgbotapi.BotAPI, ...OptFn) *Router
type IRouter interface {
	ListenUpdate(context.Context)
}

func New(logger logger.Logger) IRouter {
	return &Router{
		logger: logger,
	}
}

type Router struct {
	bot      *tgbotapi.BotAPI
	poolSize int
	logger   logger.Logger
	wg       *sync.WaitGroup
	pool     sync.Pool
	stateDB  interfaces.State

	*RouterGroup
}

type Handler func(*Ctx)

func NewRouterFactory(logger logger.Logger) RouterFactory {
	return func(bot *tgbotapi.BotAPI, options ...OptFn) *Router {
		r := &Router{logger: logger}
		r.poolSize = _poolSize
		for _, opt := range options {
			opt(r)
		}
		r.bot = bot
		r.pool.New = func() any {
			return &Ctx{bot: bot, Context: context.Background(), stateDB: r.stateDB}
		}
		r.RouterGroup = &RouterGroup{
			root:    true,
			logger:  r.logger,
			stateDB: r.stateDB,
		}
		return r
	}
}

// poolSize - default router poolSize.
const _poolSize = 100

type OptFn func(r *Router)

func WithPoolSize(psize int) OptFn {
	return func(r *Router) {
		r.poolSize = psize
	}
}

func WithState(s interfaces.State) OptFn {
	return func(r *Router) {
		r.stateDB = s
	}
}

func (r *Router) ListenUpdate(ctx context.Context) {
	updates := r.bot.GetUpdatesChan(tgbotapi.UpdateConfig{
		Offset:  0,
		Timeout: 60,
		Limit:   1000,
	})

	r.wg = &sync.WaitGroup{}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i := 1; i <= r.poolSize; i++ {
		r.wg.Add(1)
		go func(workerID int) {
			defer r.wg.Done()
			for {
				select {
				case update, ok := <-updates:
					if !ok {
						r.logger.Warn(ctx, "Update channel closed, worker shutting down",
							zap.Int("workerID", workerID))
						return
					}
					r.serveUpdate(&update)
				case <-workerCtx.Done():
					return
				}
			}
		}(i)
	}

	<-ctx.Done()
}

const shutdownPollIntervalMax = 500 * time.Millisecond

func (r *Router) Shutdown(ctx context.Context, cancel context.CancelFunc) error {
	pollIntervalBase := time.Millisecond
	nextPollInterval := func() time.Duration {
		// Add 10% letter.
		interval := pollIntervalBase + time.Duration(rand.Intn(int(pollIntervalBase/10)))
		// Double and clamp for next time.
		pollIntervalBase *= 2
		if pollIntervalBase > shutdownPollIntervalMax {
			pollIntervalBase = shutdownPollIntervalMax
		}
		return interval
	}

	r.logger.Info(ctx, "Workers, shutting down...")
	r.bot.StopReceivingUpdates()
	cancel()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	timer := time.NewTimer(nextPollInterval())

	select {
	case <-done:
		r.logger.Info(ctx, "Workers, stopped!")
		return nil
	case <-timer.C:
		r.logger.Info(ctx, "Shutdown timeout exceeded")
	}
	return nil
}

func (r *Router) Use(middlewares ...Middleware) {
	r.RouterGroup.Use(middlewares...)
}

func (r *Router) serveUpdate(update *tgbotapi.Update) {
	c := r.pool.Get().(*Ctx)
	c.update = update
	c.reset()

	r.handle(c)

	r.pool.Put(c)
}

func (r *Router) handle(c *Ctx) {
	for h := range slices.Values(r.routes) {
		r.logger.Info(c.Context, "route", zap.Any("route", h.rtype))
		if c.state == nil && h.rtype == ConversationRoute {
			r.State(c)
		}

		if h.filter(c) {
			c.handlers = h.handlers
			c.next()
			return
		}
	}
}
