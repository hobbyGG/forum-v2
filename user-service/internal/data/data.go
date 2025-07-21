package data

import (
	"context"
	"user-service/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewDB, NewRDB, NewData, NewAuthRepo)

// Data .
type Data struct {
	db  *sqlx.DB
	rdb *redis.Client
	// TODO wrapped database client
}

// NewData .
func NewData(c *conf.Data, db *sqlx.DB, rdb *redis.Client, logger log.Logger) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}
	return &Data{db: db, rdb: rdb}, cleanup, nil
}

func NewDB(c *conf.Data) *sqlx.DB {
	db := sqlx.MustConnect(c.Database.Driver, c.Database.Source)
	if err := db.Ping(); err != nil {
		panic(err)
	}
	return db
}
func NewRDB(c *conf.Data) *redis.Client {
	cli := redis.NewClient(&redis.Options{
		Network: c.Redis.Network,
		Addr:    c.Redis.Addr,
	})
	if err := cli.Ping(context.Background()).Err(); err != nil {
		panic(err)
	}
	return cli
}
