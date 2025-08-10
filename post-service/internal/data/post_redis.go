package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"post-service/internal/common"
	"post-service/internal/model"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// use redis
// 两条以内redis指令不用pipeline，三条及以上使用pipeline。

// GetPostFCByPid 从redis获取post
// 只缓存详细信息，简要信息也应从该方法获取
// 缓存未命中不会报错而是返回nil
func (repo *PostRepo) GetPostFCByPid(ctx context.Context, pid int64) (*model.Post, error) {
	// TODO:
	// 1.从redis中获取json格式的post信息
	// 2.将json信息反序列化为model.Post对象
	key := GetPostInfoKey(pid)
	postJson, err := repo.data.Rcli.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			repo.log.Errorw(
				"[repo]", "GetPostFC/Get failed",
				"err", err,
				"pid", pid,
			)
			return nil, err
		}
		// 缓存未命中，记录warn
		repo.log.Warnw(
			"[repo]", "GetPostFC/Get failed",
			"err", "cache miss",
			"pid", pid,
		)
		return nil, nil
	}
	// 缓存命中
	post := new(model.Post)
	if err := json.Unmarshal([]byte(postJson), post); err != nil {
		repo.log.Errorw(
			"[repo]", "GetPostFC",
			"err", err,
			"postJson", postJson,
		)
		return nil, err
	}

	return post, nil
}

func (repo *PostRepo) SetPostFCByPid(ctx context.Context, post *model.Post, expiration ...time.Duration) error {
	var expTime time.Duration
	switch l := len(expiration); l {
	case 0:
		// 默认情况采用轮询，从redis中顺序获取过期时间
		// 使用redis作为发号器需要考虑熔断的情况
		res, err := repo.data.Rcli.Incr(ctx, common.RKeyExpTime).Result()
		if err != nil {
			repo.log.Errorw(
				"[repo]", "SetPostFC/Incr failed",
				"err", err,
			)
			return err
		}
		// 轮询获取过期时间
		if res > 1e6 {
			// 如果超过1e6则重置为1
			_, err := repo.data.Rcli.Set(ctx, common.RKeyExpTime, 1, 0).Result()
			if err != nil {
				repo.log.Errorw(
					"[repo]", "SetPostFC/Set failed",
					"err", err,
				)
				return err
			}
		}
		expTime = time.Duration(10+res%40) * time.Minute // 10~50分钟
	case 1:
		// 有指定过期时间则用指定的过期时间
		expTime = expiration[0]
	default:
		repo.log.Errorw(
			"[repo]", "SetPostFC",
			"err", fmt.Errorf("invalid expiration length: %d", l),
		)
		return fmt.Errorf("invalid expiration length: %d", l)
	}

	key := GetPostInfoKey(post.Pid)
	postJson, err := json.Marshal(post)
	if err != nil {
		repo.log.Errorw(
			"[repo]", "SetPostFC/Marshal failed",
			"err", err,
			"key", key,
		)
		return err
	}
	if _, err := repo.data.Rcli.Set(ctx, key, postJson, expTime).Result(); err != nil {
		repo.log.Errorw(
			"[repo]", "SetPostFC/Set failed",
			"err", err,
			"key", key,
			"postJson", string(postJson),
		)
		return err
	}
	return nil
}

func (repo *PostRepo) DelPostFCByPid(ctx context.Context, pid int64) error {
	_, err := redisBreaker.Execute(func() (interface{}, error) {
		key := GetPostInfoKey(pid)
		return nil, repo.DelPostFCByKey(ctx, key)
	})
	return err
}

func (repo *PostRepo) AddHotRank(ctx context.Context, pid, score int64) error {
	_, err := redisBreaker.Execute(func() (interface{}, error) {
		// 将帖子加入热度排行榜
		if err := repo.data.Rcli.ZAdd(ctx, common.RKeyPostHotRank, redis.Z{
			Score:  float64(score),
			Member: pid,
		}).Err(); err != nil {
			repo.log.Errorw(
				"[repo]", "AddHotRank/ZAdd failed",
				"err", err,
				"pid", pid,
			)
			return nil, err
		}
		return nil, nil
	})
	return err
}

func (repo *PostRepo) DelHotRankMem(ctx context.Context, pid int64) error {
	// 从热度排行榜中删除帖子
	if err := repo.data.Rcli.ZRem(ctx, common.RKeyPostHotRank, pid).Err(); err != nil {
		repo.log.Errorw(
			"[repo]", "DelHotRank/ZRem failed",
			"err", err,
			"pid", pid,
		)
		return err
	}
	return nil
}

func (repo *PostRepo) DelPostFCByKey(ctx context.Context, key string) error {
	if err := repo.data.Rcli.Del(ctx, key).Err(); err != nil {
		repo.log.Errorw(
			"[repo]", "DelPostFCByKey/Del failed",
			"err", err,
			"key", key,
		)
		return err
	}
	return nil
}

func (repo *PostRepo) ListPostPreviewByHot(ctx context.Context, page, pageSize int64) ([]*model.PostPreview, error) {
	// TODO:
	// 1.从数据库中获取热度最高的post信息
	// 2.将获取到的信息封装成model.PostPreview对象
	start := page * pageSize
	end := (page+1)*pageSize - 1
	members, err := repo.data.Rcli.ZRangeWithScores(ctx, common.RKeyPostHotRank, start, end).Result()
	if err != nil {
		repo.log.Errorw(
			"[repo]", "ListPostPreviewByHot/ZRangeWithScores failed",
			"err", err,
			"page", page,
			"pageSize", pageSize,
		)
		return nil, err
	}

	PostPreviews := make([]*model.PostPreview, 0, len(members))
	for _, member := range members {
		// 从数据库获取详细数据
		pidStr, ok := member.Member.(string)
		if !ok {
			repo.log.Errorw(
				"[repo]", "ListPostPreviewByHot/Member type assertion failed",
				"err", err,
				"member", member.Member,
			)
			return nil, errors.New("type error")
		}
		pid, err := strconv.ParseInt(pidStr, 10, 64)
		if err != nil {
			repo.log.Errorw(
				"[repo]", "ListPostPreviewByHot/Member type assertion failed",
				"err", err,
				"member", member.Member,
			)
		}
		post, err := repo.GetPostById(ctx, pid)
		if err != nil {
			repo.log.Errorw(
				"[repo]", "ListPostPreviewByHot/GetPostById failed",
				"err", err,
				"pid", pid,
			)
			return nil, err
		}
		PostPreviews = append(PostPreviews, post.ToPreview())
	}

	return PostPreviews, nil
}

func (repo *PostRepo) ExistedRsetMem(ctx context.Context, key string, mem any) (bool, error) {
	return repo.data.Rcli.SIsMember(ctx, key, mem).Result()
}

func (repo *PostRepo) AddRsetMem(ctx context.Context, key string, mem any) error {
	return repo.data.Rcli.SAdd(ctx, key, mem).Err()
}

func (repo *PostRepo) DelRsetMem(ctx context.Context, key string, mem any) error {
	return repo.data.Rcli.SRem(ctx, key, mem).Err()
}

func GetPostInfoKey(pid int64) string {
	return fmt.Sprintf(common.RKeyPostPrefix+"%d", pid)
}

// ----------------------------------------------
// 其他服务

func delCacheAfterWrite(ctx context.Context, repo *PostRepo, pid int64) error {
	// 写后删除策略
	// 失败后重试3次
	// 如果重试失败则存入消息队列，由job处理
	if err := repo.DelPostFCByPid(ctx, pid); err != nil {
		repo.log.Errorw(
			"[repo]", "CreatePost/DelPostFC failed",
			"pid", pid,
			"err", err,
		)
		for range 3 {
			if err = repo.DelPostFCByPid(ctx, pid); err == nil {
				return nil
			}
			repo.log.Errorw(
				"[repo]", "CreatePost/DelPostFC retry failed",
				"err", err,
			)
		}
		// 存入某个消息队列kafka/pulsar 做延迟删除策略
		if err := repo.data.KafkaW.WriteMessages(ctx, kafka.Message{
			Key:   common.KKeyKDelInfo,
			Value: []byte(GetPostInfoKey(pid)),
			Topic: common.TopicPostCacheDel,
		}); err != nil {
			repo.log.Errorw(
				"[repo]", "CreatePost/writeMessages failed",
				"err", err,
			)
			return err
		}
		repo.log.Errorw(
			"[repo]", "CreatePost/DelPostFC retry failed after 3 times, storing in message queue",
			"err", err,
			"pid", pid,
		)
		return err
	}
	return nil
}

// 缓存预热
func (repo *PostRepo) WarmUpPostCache(ctx context.Context) {
	// 读取pg中热度前100的帖子，并且只需要pid与score
	// 将kv对存入redis中
	hotMap := repo.GetRankFromPG(ctx, 100)
	for pid, score := range hotMap {
		if err := repo.AddHotRank(ctx, pid, score); err != nil {
			repo.log.Errorw(
				"[repo]", "WarmUpPostCache/AddHotRank failed",
				"err", err,
				"pid", pid,
				"score", score,
			)
			panic(err)
		}
	}
	go repo.WarmUpPostCacheAll(ctx)
	go repo.SyncRank(ctx)
}

func (repo *PostRepo) WarmUpPostCacheAll(ctx context.Context) {
	// 读取pg中所有的帖子
	// 将kv对存入redis中
	hotMap := repo.GetRankFromPG(ctx)
	for pid, score := range hotMap {
		if err := repo.AddHotRank(ctx, pid, score); err != nil {
			repo.log.Errorw(
				"[repo]", "WarmUpPostCache/AddHotRank failed",
				"err", err,
				"pid", pid,
				"score", score,
			)
			return
		}
	}
}

func (repo *PostRepo) SyncRank(ctx context.Context) {
	timer := time.NewTimer(time.Minute * 30) //30分钟同步一次
	for {
		<-timer.C
		go repo.WarmUpPostCacheAll(ctx)
		timer.Reset(time.Minute * 30)
	}
}
