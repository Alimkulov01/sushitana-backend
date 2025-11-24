package migration

import (
	"context"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/golang-migrate/migrate/v4/source/github"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"sushitana/pkg/config"
	"sushitana/pkg/logger"
)

var Module = fx.Options(
	fx.Invoke(New),
)

type Params struct {
	fx.In
	Logger logger.Logger
	Config config.IConfig
}

func New(p Params) {
	ctx := context.TODO()

	m, err := migrate.New("file://migrations", p.Config.GetString("database.migration"))
	if err != nil {
		p.Logger.Error(ctx, "err from migration.New", zap.Error(err))
		os.Exit(1)
		return
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		p.Logger.Error(ctx, "err from up migration", zap.Error(err))
		os.Exit(1)
		return
	}
}
