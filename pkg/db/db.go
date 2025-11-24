package db

import (
	"context"

	"sushitana/pkg/logger"

	"sushitana/pkg/config"
	"sushitana/pkg/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Options(
	fx.Provide(NewDBConn),
)

type Querier interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, optionsAndArgs ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, optionsAndArgs ...interface{}) pgx.Row
	SendBatch(context.Context, *pgx.Batch) pgx.BatchResults
}

type Params struct {
	fx.In
	Config config.IConfig
	Logger logger.Logger
}

type dbConn struct {
	config config.IConfig
	dbPool *pgxpool.Pool
	logger logger.Logger
}

func NewDBConn(params Params) (Querier, error) {

	var (
		dns = params.Config.GetString("database.dns")
		err error
		ctx = context.Background()
	)

	db, err := pgxpool.New(context.Background(), dns)
	if err != nil {
		params.Logger.Error(ctx, "Err on pgxpool.Connect", zap.Error(err), zap.String("dns", dns))
		return nil, err
	}

	err = db.Ping(context.Background())
	if err != nil {
		params.Logger.Error(ctx, "Err on db.Ping", zap.Error(err), zap.String("dns", dns))
		return nil, err
	}

	params.Logger.Info(ctx, "DB: Connected successfully", zap.String("dns", dns))

	return &dbConn{
		dbPool: db,
		logger: params.Logger,
		config: params.Config,
	}, nil
}

func (db *dbConn) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	db.logger.Info(ctx, "DB: Exec sql", zap.String("sql", utils.RemoveSpecialChars2(sql)), zap.Any("args", args))
	return db.dbPool.Exec(ctx, sql, args...)
}

func (db *dbConn) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	db.logger.Debug(ctx, "DB: Query sql", zap.String("sql", utils.RemoveSpecialChars2(sql)), zap.Any("args", args))
	return db.dbPool.Query(ctx, sql, args...)
}

func (db *dbConn) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	db.logger.Info(ctx, "DB: QueryRow sql", zap.String("sql", utils.RemoveSpecialChars2(sql)), zap.Any("args", args))
	return db.dbPool.QueryRow(ctx, sql, args...)
}

func (db *dbConn) Begin(ctx context.Context) (pgx.Tx, error) {
	return db.dbPool.Begin(ctx)
}

func (db *dbConn) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	return db.dbPool.SendBatch(ctx, batch)
}
