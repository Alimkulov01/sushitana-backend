package cache

import (
	"sync"

	"sushitana/pkg/logger"
	"sushitana/pkg/utils"

	"go.uber.org/fx"
)

var (
	Module   = fx.Provide(New)
	MemCache = map[string][]byte{}
)

type (
	Params struct {
		fx.In
		Logger logger.Logger
	}

	ICache interface {
		Set(key string, value interface{}) error
		Get(key string) (interface{}, error)
		Delete(key string) error
		SaveObj(key string, value interface{}) error
		GetObj(key string, value interface{}) error
	}

	cache struct {
		logger   logger.Logger
		expires  map[string]int64
		memCache map[string][]byte
		m        sync.RWMutex
	}
)

func New(p Params) ICache {
	return &cache{
		logger:   p.Logger,
		memCache: MemCache,
		expires:  map[string]int64{},
		m:        sync.RWMutex{},
	}
}

func (c *cache) Set(key string, value interface{}) error {
	c.m.Lock()
	defer c.m.Unlock()

	c.memCache[key] = utils.Marshal(value)
	return nil
}

func (c *cache) Get(key string) (interface{}, error) {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.memCache[key], nil
}

func (c *cache) Delete(key string) error {
	c.m.Lock()
	defer c.m.Unlock()

	delete(c.memCache, key)
	return nil
}

func (c *cache) SaveObj(key string, value interface{}) error {
	c.m.Lock()
	defer c.m.Unlock()

	c.memCache[key] = utils.Marshal(value)
	return nil
}

func (c *cache) GetObj(key string, value interface{}) error {
	c.m.RLock()
	defer c.m.RUnlock()

	cacheVal := c.memCache[key]

	return utils.Unmarshal(cacheVal, &value)
}
