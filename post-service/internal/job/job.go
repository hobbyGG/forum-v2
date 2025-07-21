package job

import (
	"post-service/internal/common"
	"post-service/internal/conf"
	"post-service/internal/data"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/segmentio/kafka-go"
)

var ProviderSet = wire.NewSet(NewJobRepo)

type JobRepo struct {
	repo   *data.PostRepo
	data   *data.Data
	kafkaR map[string]*kafka.Reader
	log    *log.Helper
}

func NewJobRepo(c *conf.Data, repo *data.PostRepo, data *data.Data, logger log.Logger) *JobRepo {
	partitions, err := data.KafkaConn.ReadPartitions()
	if err != nil {
		panic(err)
	}
	m := make(map[string]struct{}, len(partitions))
	for _, p := range partitions {
		m[p.Topic] = struct{}{}
	}

	kafkaR := make(map[string]*kafka.Reader, len(m))
	for topic := range m {
		temp := kafka.NewReader(kafka.ReaderConfig{
			Brokers: c.Kafka.Addrs,
			Topic:   topic,
		})
		if temp == nil {
			panic("kafka reader is nil")
		}
		kafkaR[topic] = temp
	}
	if _, ok := kafkaR[common.TopicPostCacheDel]; !ok {
		log.Warnw(
			"[job]", "DelayDeleteJob/TopicPostCacheDel not found",
		)
		data.KafkaConn.CreateTopics(
			kafka.TopicConfig{
				Topic: common.TopicPostCacheDel,
				ConfigEntries: []kafka.ConfigEntry{
					{
						ConfigName:  "retention.ms",
						ConfigValue: "60000",
					},
				},
			},
		)
		kafkaR[common.TopicPostCacheDel] = kafka.NewReader(kafka.ReaderConfig{
			Brokers: c.Kafka.Addrs,
			Topic:   common.TopicPostCacheDel,
		})
	}
	return &JobRepo{
		repo:   repo,
		data:   data,
		kafkaR: kafkaR,
		log:    log.NewHelper(logger),
	}
}
