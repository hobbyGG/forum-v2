package data

import (
	"context"
	"post-service/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewPostRepo, NewPostRepoForJob)

// Data .
type Data struct {
	// TODO wrapped database client
	MySqlCli *sqlx.DB
	PgxCli   *pgxpool.Pool
	Rcli     *redis.Client

	KafkaW    *kafka.Writer
	KafkaConn *kafka.Conn
}

// NewData .
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {

	// mysql
	mysqlCli, err := sqlx.Connect(c.Mysql.Driver, c.Mysql.Source)
	if err != nil {
		panic(err)
	}

	// pg
	pgxCli, err := pgxpool.New(context.Background(), c.Pg.Source)
	if err != nil {
		panic(err)
	}
	err = pgxCli.Ping(context.Background())
	if err != nil {
		panic(err)
	}

	// redis
	rCli := redis.NewClient(&redis.Options{
		Addr:         c.Redis.Addr,
		ReadTimeout:  c.Redis.ReadTimeout.AsDuration(),
		WriteTimeout: c.Redis.WriteTimeout.AsDuration(),
	})
	err = rCli.Ping(context.Background()).Err()
	if err != nil {
		panic(err)
	}

	kafkaW, kafkaConn := NewKafka(c)

	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
		if err := mysqlCli.Close(); err != nil {
			log.NewHelper(logger).Error("failed to close mysql client", err)
		}
		if err := rCli.Close(); err != nil {
			log.NewHelper(logger).Error("failed to close redis client", err)
		}
		if err := kafkaConn.Close(); err != nil {
			log.NewHelper(logger).Error("failed to close kafka connection", err)
		}
		if err := kafkaW.Close(); err != nil {
			log.NewHelper(logger).Error("failed to close kafka writer", err)
		}
	}

	return &Data{MySqlCli: mysqlCli, PgxCli: pgxCli, Rcli: rCli, KafkaW: kafkaW, KafkaConn: kafkaConn}, cleanup, nil
}

func NewKafka(c *conf.Data) (*kafka.Writer, *kafka.Conn) {
	conn, err := kafka.Dial("tcp", c.Kafka.Addrs[0])
	if err != nil {
		panic(err)
	}
	hcTopicConfig := kafka.TopicConfig{
		Topic:             "health-check",
		NumPartitions:     1,
		ReplicationFactor: 1,
		ConfigEntries: []kafka.ConfigEntry{
			{
				ConfigName:  "retention.ms",
				ConfigValue: "60000",
			},
		},
	}
	partitions, err := conn.ReadPartitions()
	if err != nil {
		panic(err)
	}

	m := make(map[string]struct{}, len(partitions))
	for _, partition := range partitions {
		m[partition.Topic] = struct{}{}
	}
	if _, ok := m[hcTopicConfig.Topic]; !ok {
		if err := conn.CreateTopics(hcTopicConfig); err != nil {
			panic(err)
		}
	}

	w := &kafka.Writer{
		Addr:                   kafka.TCP(c.Kafka.Addrs...),
		Balancer:               &kafka.LeastBytes{},
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: true,
	}
	if err := w.WriteMessages(context.Background(),
		kafka.Message{
			Topic: "health-check",
			Value: []byte("health check"),
		},
	); err != nil {
		panic(err)
	}

	return w, conn
}
