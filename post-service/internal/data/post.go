package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"post-service/internal/biz"
	"post-service/internal/common"
	"post-service/internal/model"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/sony/gobreaker"
	"golang.org/x/sync/singleflight"
)

// data implements basic operations for biz.
// data should not contain any business logic.
// data should only focus on data interaction, such as database operations, cache operations, etc.

const (
	StrAccessPG = "access_pg"
	StrHot      = "hot"
	StrNew      = "new"
)

var (
	sfg     = singleflight.Group{}
	breaker = gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "DataRepoBreaker"})
)

type PostRepo struct {
	data *Data
	log  *log.Helper
}

func NewPostRepo(data *Data, logger log.Logger) biz.PostRepo {
	return &PostRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func NewPostRepoForJob(data *Data, logger log.Logger) *PostRepo {
	return &PostRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// ----------------------------------------------
// use pg
// redis与pg的数据库一致性采用写后删策略：
// 任何写入操作成功后应删除对应redis缓存
// 仍和读操作未命中都要去查询数据库
// 查询得到的数据存入缓存中
// 需要考虑redis崩溃或数据库崩溃的情况

func (repo *PostRepo) CreatePost(ctx context.Context, post *model.CreatePostParam) (*model.Post, error) {
	sqlStr := `
	insert into post_info(pid, title, content, author, uid, score, tags)
	values($1, $2, $3, $4, $5, $6, $7)
	returning id, pid, is_del, create_time, update_time, title, content, author, uid, status, score, tags, view, "like";`
	replyPost := new(model.Post)

	res, err := breaker.Execute(func() (interface{}, error) {
		if err := repo.data.PgxCli.QueryRow(ctx, sqlStr, post.ToArgs()...).
			Scan(replyPost.ScanArgs()...); err != nil {
			repo.log.Errorw(
				"[repo]", "CreatePost/QueryRow failed",
				"err", err,
				"postParam", post,
			)
			return nil, err
		}

		if err := delCacheAfterWrite(ctx, repo, post.Pid); err != nil {
			repo.log.Errorw(
				"[repo]", "CreatePost/DelPostFC failed",
				"err", err,
				"pid", post.Pid)
		}
		return replyPost, nil
	})
	return res.(*model.Post), err
}

func (repo *PostRepo) UpdatePost(ctx context.Context, post *model.UpdatePostParam) (*model.Post, error) {
	// 做param的指针检查
	if post.Title == nil || post.Content == nil || post.Status == nil || post.Score == nil || post.Tags == nil {
		repo.log.Errorw(
			"[repo]", "UpdatePost/Param check failed",
			"err", errors.New("param pointer is nil"),
			"title", post.Title,
			"content", post.Content,
			"status", post.Status,
			"score", post.Score,
			"tags", post.Tags,
		)
		return nil, errors.New("param pointer is nil")
	}

	sqlStr := `
	update post_info
	set pid = $1, title = $2, content = $3, status = $4, score = $5, tags = $6
	where pid = $7
	returning id, pid, is_del, create_time, update_time, title, content, author, uid, status, score, tags, view, "like";`

	res, err := breaker.Execute(func() (interface{}, error) {
		replyPost := new(model.Post)
		agrs := append(post.ToArgs(), post.Pid)
		if err := repo.data.PgxCli.QueryRow(ctx, sqlStr, agrs...).Scan(replyPost.ScanArgs()...); err != nil {
			repo.log.Errorw(
				"[repo]", "UpdatePost/QueryRow failed",
				"err", err,
			)
			return nil, err
		}
		return replyPost, nil
	})
	if err != nil {
		return nil, err
	}

	// 写删除
	if err := delCacheAfterWrite(ctx, repo, post.Pid); err != nil {
		repo.log.Errorw(
			"[repo]", "UpdatePost/DelPostFC failed",
			"err", err,
			"pid", post.Pid,
		)
	}
	return res.(*model.Post), nil
}

func (repo *PostRepo) DeletePost(ctx context.Context, id int64) error {
	// 实现逻辑删除
	sqlStr := `
	update post_info
	set is_del = null
	where pid = $1`

	_, err := breaker.Execute(func() (interface{}, error) {
		if _, err := repo.data.PgxCli.Exec(ctx, sqlStr, id); err != nil {
			repo.log.Errorw(
				"[repo]", "DeletePost/Exec failed",
				"err", err,
				"pid", id,
			)
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		return err
	}

	if err := delCacheAfterWrite(ctx, repo, id); err != nil {
		repo.log.Errorw(
			"[repo]", "DeletePost/DelPostFC failed",
			"err", err,
			"pid", id,
		)
	}
	return nil
}

func (repo *PostRepo) GetPostById(ctx context.Context, pid int64) (*model.Post, error) {
	// 先读取缓存中的数据
	// 如果未命中则查数据库
	// 将查到的结果存入缓存中
	// 要求：防止缓存击穿、缓存穿透
	postCache, err := repo.GetPostFCByPid(ctx, pid)
	if err != nil {
		// 查缓存错误
		repo.log.Errorw(
			"[repo]", "GetPostById/GetPostFCByPid failed",
			"err", err,
			"pid", pid,
		)
	}
	if postCache != nil {
		// 缓存命中直接返回
		return postCache, nil
	}

	// 缓存未命中或redis出错
	// 使用singleflight防止缓存击穿
	post, err, _ := sfg.Do(StrAccessPG, func() (interface{}, error) {
		sqlStr := `
		select id, pid, is_del, create_time, update_time, title, content, author, uid, status, score, tags, view, "like"
		from post_info where pid = $1 and is_del = 0`
		post := new(model.Post)
		var retryErr error
		for range 3 {
			if retryErr = repo.data.PgxCli.QueryRow(ctx, sqlStr, pid).Scan(post.ScanArgs()...); retryErr != nil {
				repo.log.Errorw(
					"[repo]", "GetPostById/QueryRow failed",
					"err", retryErr,
					"pid", pid,
				)
				continue
			}
			break
		}
		if retryErr != nil {
			repo.log.Errorw(
				"[repo]", "GetPostById/QueryRow retry failed",
				"err", retryErr,
				"pid", pid,
			)
			return nil, retryErr
		}

		// 查询成功后将数据存入缓存
		if err := repo.SetPostFCByPid(ctx, post); err != nil {
			repo.log.Errorw(
				"[repo]", "GetPostById/SetPostFCByPid failed",
				"err", err,
				"pid", pid,
			)
			return nil, err
		}
		return post, nil
	})

	if err != nil {
		return nil, err
	}

	return post.(*model.Post), nil
}

func (repo *PostRepo) ListPostPreview(ctx context.Context, page, pageSize int64) ([]*model.PostPreview, error) {
	// 可选：增加列表缓存，已经定义的参数(key)
	sqlStr := `
	select id, pid, create_time, update_time, title, content, author, status, score, tags, view, "like"
	from post_info 
	where is_del = 0
	limit $1 offset $2`

	rows, err := repo.data.PgxCli.Query(ctx, sqlStr, pageSize, page*pageSize)
	if err != nil {
		repo.log.Errorw(
			"[repo]", "ListPosts/Query failed",
			"err", err,
		)
		return nil, err
	}
	defer rows.Close()

	posts := make([]*model.PostPreview, 0, pageSize)
	for rows.Next() {
		temp := new(model.PostPreview)
		if err := rows.Scan(temp.ScanArgs()...); err != nil {
			repo.log.Errorw(
				"[repo]", "ListPosts/Scan failed",
				"err", err,
			)
			return nil, err
		}
		posts = append(posts, temp)
	}
	return posts, nil
}

// ----------------------------------------------

// ----------------------------------------------
// use mysql

func (repo *PostRepo) GetUserByUid(ctx context.Context, uid int64) (*model.User, error) {
	sqlStr := `
	select id, uid, is_del, user_name, create_at, update_at
	from login_info
	where uid = ? and is_del = 0`
	user := new(model.User)
	err := repo.data.MySqlCli.GetContext(ctx, user, sqlStr, uid)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// ----------------------------------------------

// ----------------------------------------------
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
	_, err := breaker.Execute(func() (interface{}, error) {
		key := GetPostInfoKey(pid)
		return nil, repo.DelPostFCByKey(ctx, key)
	})
	return err
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

func GetPostInfoKey(pid int64) string {
	return fmt.Sprintf(common.RKeyPostPrefix+"%d", pid)
}

// ----------------------------------------------

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
