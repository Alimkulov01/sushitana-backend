package pkg

import (
	"go.uber.org/fx"

	"sushitana/pkg/cache"
	"sushitana/pkg/config"
	"sushitana/pkg/db"
	"sushitana/pkg/filemanager"
	"sushitana/pkg/logger"
	"sushitana/pkg/migration"
	"sushitana/pkg/redis"
	"sushitana/pkg/reply"
	"sushitana/pkg/repository"
	"sushitana/pkg/tgrouter"
)

var Module = fx.Options(
	config.Module,
	logger.Module,
	migration.Module,
	repository.Module,
	db.Module,
	cache.Module,
	reply.Module,
	filemanager.Module,
	tgrouter.Module,
	redis.Module,
)
