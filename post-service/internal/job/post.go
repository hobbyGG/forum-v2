package job

import (
	"context"
	"post-service/internal/common"
)

func (j *JobRepo) PostInfoJob(ctx context.Context) {
	j.log.Infof("Delaying delete job started")

	for {
		// 消费 Kafka 消息
		// 删除消息中指定的cache
		r := j.kafkaR[common.TopicPostCacheDel]
		msg, err := r.ReadMessage(ctx)
		if err != nil {
			j.log.Errorw(
				"[job]", "DelayDeleteJob/ReadMessage failed",
				"err", err,
			)
			continue
		}
		switch string(msg.Key) {
		case string(common.KKeyKDelInfo):
			// 删除缓存
			if err := j.repo.DelPostFCByKey(ctx, string(msg.Value)); err != nil {
				j.log.Errorw(
					"[job]", "DelayDeleteJob/DelPostFCByKey failed",
					"err", err,
					"key", string(msg.Value),
				)
			}
		}
	}

}
