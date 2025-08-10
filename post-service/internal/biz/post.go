package biz

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"post-service/internal/common"
	"post-service/internal/conf"
	"post-service/internal/model"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/go-kratos/kratos/v2/log"
	jwtkratos "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
)

type PostRepo interface {
	CreatePost(ctx context.Context, post *model.CreatePostParam) (*model.Post, error)
	UpdatePost(ctx context.Context, post *model.UpdatePostParam) (*model.Post, error)
	DeletePost(ctx context.Context, id int64) error
	GetPostById(ctx context.Context, id int64) (*model.Post, error)
	ListPostPreviewByTime(ctx context.Context, page, pageSize int64) ([]*model.PostPreview, error)
	ListPostPreviewByHot(ctx context.Context, page, pageSize int64) ([]*model.PostPreview, error)
	ListPostPreviewByHotFallback(ctx context.Context, page, pageSize int64) ([]*model.PostPreview, error)

	GetUserByUid(ctx context.Context, uid int64) (*model.User, error)

	ExistedRsetMem(ctx context.Context, key string, mem any) (bool, error)
	AddRsetMem(ctx context.Context, key string, mem any) error
	DelRsetMem(ctx context.Context, key string, mem any) error

	RedisClient() *redis.Client
}

type redisLocker struct {
	Client *redis.Client
	ttl    time.Duration
	id     int64
}

type PostUsecase struct {
	repo    PostRepo
	rlocker *redisLocker
	node    *snowflake.Node
	log     log.Helper
}

func NewPostUsecase(c *conf.Biz, repo PostRepo, logger log.Logger) *PostUsecase {
	start, err := time.Parse("2006-01-02 15:04:05", c.App.StartTime)
	if err != nil {
		panic(err)
	}
	snowflake.Epoch = start.UnixNano() / 1e6 // 设置雪花算法的起始时间戳为配置中的时间
	node, err := snowflake.NewNode(c.App.Machine_ID)
	if err != nil {
		panic(err)
	}

	rlocker := &redisLocker{Client: repo.RedisClient(), ttl: 5 * time.Second, id: c.App.Machine_ID}

	return &PostUsecase{
		repo:    repo,
		node:    node,
		log:     *log.NewHelper(logger),
		rlocker: rlocker,
	}
}

type Claims struct {
	UID int64 `json:"uid"`
	jwt.RegisteredClaims
}

const (
	StrTime = "time"
	StrHot  = "hot"
)

var (
	errTokenParase = errors.New("token is invalid")
	errToekenType  = errors.New("token type is invalid")

	errPostNotExisted = errors.New("post not existed")

	errInvalideParam = errors.New("invalid Param")
)

func (uc *PostUsecase) CreatePost(ctx context.Context, param *model.CreatePostParam) (*model.Post, error) {
	// 通过uid获取user信息
	claimsToken, ok := jwtkratos.FromContext(ctx)
	if !ok {
		uc.log.Errorw(
			"[biz]", "CreatePost/FromContext failed",
			"err", errTokenParase,
		)
		return nil, errTokenParase
	}
	claims, ok := claimsToken.(*Claims)
	if !ok {
		return nil, errToekenType
	}
	uid := claims.UID
	user, err := uc.repo.GetUserByUid(ctx, uid)
	if err != nil {
		uc.log.Errorw(
			"[biz]", "CreatePost/GetUserByUid failed",
			"err", err,
		)
		return nil, err
	}

	// 补全param
	param.Uid = uid
	param.Author = user.Username
	param.Score = time.Now().UnixNano()
	param.Pid = uc.node.Generate().Int64()

	// 同步进数据库中
	uc.log.WithContext(ctx).Infof("Creating post with title: %s by userID: %d", param.Title, uid)
	post, err := uc.repo.CreatePost(ctx, param)
	if err != nil {
		uc.log.Errorw(
			"[biz]", "CreatePost/CreatePost failed",
			"err", err,
		)
		return nil, err
	}

	return post, nil
}

// UpdatePost 更新帖子，必须填写pid字段，否则会返回err
func (uc *PostUsecase) UpdatePost(ctx context.Context, param *model.UpdatePostParam) (*model.Post, error) {
	if param.Pid == 0 {
		uc.log.Errorw(
			"[biz]", "UpdatePost/Pid is required",
			"err", errPostNotExisted,
		)
		return nil, errPostNotExisted
	}
	// 检查该post是否存在
	// 上锁
	keyLock := fmt.Sprintf(common.RKeyPostLock, param.Pid)
	maxTry := 11
	for i := 0; i < maxTry; i++ {
		if i == maxTry-1 {
			// 获取锁失败
			uc.log.Errorw(
				"[biz]", "UpdatePost/Lock failed",
				"err", fmt.Errorf("failed to acquire lock after %d attempts", maxTry),
				"pid", param.Pid,
			)
			return nil, fmt.Errorf("failed to acquire lock after %d attempts", maxTry)
		}
		waitTime := 1
		ok, err := uc.rlocker.Lock(ctx, keyLock)
		if err != nil {
			// 上锁时出错
			uc.log.Errorw(
				"[biz]", "UpdatePost/Lock failed",
				"err", err,
			)
			return nil, err
		}
		if !ok {
			// 锁正在使用，等待指数秒后重试
			time.Sleep(time.Duration(waitTime) * time.Millisecond)
			waitTime *= 2
			continue
		}
		break
	}
	// 成功获取锁

	postInDB, err := uc.repo.GetPostById(ctx, param.Pid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			uc.log.Errorw(
				"[biz]", "UpdatePost/GetPostById failed",
				"err", errPostNotExisted,
				"pid", param.Pid,
			)
			return nil, errPostNotExisted
		}
		uc.log.Errorw(
			"[biz]", "UpdatePost/GetPostById failed",
			"err", err,
			"pid", param.Pid,
		)
		return nil, err
	}
	if postInDB == nil {
		uc.log.Errorw(
			"[biz]", "UpdatePost/PostInDB is nil",
			"err", errPostNotExisted,
			"pid", param.Pid,
		)
		return nil, errPostNotExisted
	}

	// 如果存在，则填补剩余为空的字段
	if param.Title == nil {
		param.Title = &postInDB.Title
	}
	if param.Content == nil {
		param.Content = &postInDB.Content
	}
	if param.Status == nil {
		param.Status = &postInDB.Status
	}
	if param.Score == nil {
		param.Score = &postInDB.Score
	}
	if param.Tags == nil {
		param.Tags = postInDB.Tags
	}
	if param.Like == nil {
		param.Like = &postInDB.Like
	}

	uc.log.WithContext(ctx).Infof("Updating post with PID: %d", param.Pid)
	post, err := uc.repo.UpdatePost(ctx, param)
	if err := uc.rlocker.Unlock(ctx, keyLock); err != nil {
		uc.log.Errorw(
			"[biz]", "UpdatePost/Unlock failed",
			"err", err,
		)
		return nil, err
	}
	if err != nil {
		uc.log.Errorw(
			"[biz]", "UpdatePost/UpdatePost failed",
			"err", err,
		)
		return nil, err
	}
	return post, nil
}

func (uc *PostUsecase) DeletePost(ctx context.Context, id int64) error {
	// 检查是否存在，如果不存在则返回错误
	_, err := uc.repo.GetPostById(ctx, id)
	if err != nil {
		// NotFound与其他错误
		if errors.Is(err, sql.ErrNoRows) {
			uc.log.Errorw(
				"[biz]", "DeletePost/GetPostById failed",
				"err", errPostNotExisted,
			)
			return errPostNotExisted
		}
		uc.log.Errorw(
			"[biz]", "DeletePost/GetPostById failed",
			"err", err,
		)
		return err
	}

	uc.log.WithContext(ctx).Infof("Deleting post with PID: %d", id)
	if err := uc.repo.DeletePost(ctx, id); err != nil {
		uc.log.Errorw(
			"[biz]", "DeletePost/DeletePost failed",
			"err", err,
		)
		return err
	}

	return nil
}

func (uc *PostUsecase) GetPostById(ctx context.Context, id int64) (*model.Post, error) {
	uc.log.WithContext(ctx).Infof("Fetching post with PID: %d", id)
	post, err := uc.repo.GetPostById(ctx, id)
	if err != nil {
		uc.log.Errorw(
			"[biz]", "GetPostById/GetPostById failed",
			"err", err,
		)
		return nil, err
	}
	return post, nil
}

func (uc *PostUsecase) ListPostPreview(ctx context.Context, page, pageSize int64, searchType *string) ([]*model.PostPreview, error) {
	if searchType == nil {
		// 默认按时间查询
		searchType = new(string)
		*searchType = StrTime
	}
	switch *searchType {
	case StrTime:
		uc.log.WithContext(ctx).Infof("Listing posts with page order by time: %d, pageSize: %d ", page, pageSize)
		posts, err := uc.repo.ListPostPreviewByTime(ctx, page, pageSize)
		if err != nil {
			uc.log.Errorw(
				"[biz]", "ListPostPreview/ListPostPreview failed",
				"err", err,
			)
			return nil, err
		}
		return posts, nil
	case StrHot:
		uc.log.WithContext(ctx).Infof("Listing posts with page order by hot: %d, pageSize: %d ", page, pageSize)
		posts, err := uc.repo.ListPostPreviewByHot(ctx, page, pageSize)
		if err != nil {
			uc.log.Errorw(
				"[biz]", "ListPostPreview/ListPostByHot failed",
				"err", err,
			)
			if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
				fallPosts, err := uc.repo.ListPostPreviewByHotFallback(ctx, page, pageSize)
				if err != nil {
					uc.log.Errorw(
						"biz", "ListPostPreview/ListPostPreviewByHotFallback failed",
						"err", err,
					)
					return nil, err
				}
				return fallPosts, nil
			}
			return nil, err
		}
		return posts, nil
	}
	uc.log.WithContext(ctx).Errorw(
		"[biz]", "ListPostPreview/Unknown search type",
		"searchType", *searchType,
	)
	return nil, errors.New("unkown error")
}

func (uc *PostUsecase) AddPostLike(ctx context.Context, pid int64, like int32) (*model.Post, error) {
	// 检查帖子是否存在
	postInDB, err := uc.repo.GetPostById(ctx, pid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			uc.log.Errorw(
				"[biz]", "UpdatePost/GetPostById failed",
				"err", errPostNotExisted,
				"pid", pid,
			)
			return nil, errPostNotExisted
		}
		uc.log.Errorw(
			"[biz]", "UpdatePost/GetPostById failed",
			"err", err,
			"pid", pid,
		)
		return nil, err
	}
	if postInDB == nil {
		uc.log.Errorw(
			"[biz]", "UpdatePost/PostInDB is nil",
			"err", errPostNotExisted,
			"pid", pid,
		)
		return nil, errPostNotExisted
	}

	// 存在则更新点赞情况
	// like = 1时
	// 如果已经点赞过，再次点赞则不做任何操作
	// 如果未点赞过，记录该点赞并更新分数到redis与pg中
	// like = 0时
	// 如果已经点赞过，取消点赞并更新分数到redis与pg中
	// 如果未点赞过，则不做任何操作
	uid, err := GetUidFromCtx(ctx)
	if err != nil {
		uc.log.Errorw(
			"[biz]", "AddPostLike/GetUidFromCtx failed",
			"err", err,
		)
		return nil, err
	}

	key := fmt.Sprintf(common.RKeyPostLike, pid)
	// 取消点赞操作，检查是否点过赞
	ok, err := uc.repo.ExistedRsetMem(ctx, key, uid)
	if err != nil {
		uc.log.Errorw(
			"[biz]", "AddPostLike/ExistedRsetMem failed",
			"err", err,
		)
		return nil, err
	}

	var newlike int64
	if like == 1 {
		// 点赞操作，检查是否点过赞
		if ok {
			return postInDB, nil // 已经点过赞，直接返回
		}
		// 未点过赞，更新redis
		if err := uc.repo.AddRsetMem(ctx, key, uid); err != nil {
			uc.log.Errorw(
				"[biz]", "AddPostLike/AddRsetMem failed",
				"err", err,
				"pid", pid,
			)
			return nil, err
		}
		newlike = postInDB.Like + 1
	}
	if like == 0 {
		if !ok {
			return postInDB, nil // 未点过赞，直接返回
		}
		// 已经点过赞，更新redis
		if err := uc.repo.DelRsetMem(ctx, key, uid); err != nil {
			uc.log.Errorw(
				"[biz]", "AddPostLike/DelRsetMem failed",
				"err", err,
				"pid", pid,
			)
			return nil, err
		}
		newlike = postInDB.Like - 1
		if newlike < 0 {
			newlike = 0 // 确保点赞数不为负数
		}
	}
	if like != 0 && like != 1 {
		uc.log.Errorw(
			"[biz]", "AddPostLike/Invalid like value",
			"err", errInvalideParam,
			"like", like,
		)
		return nil, errInvalideParam
	}

	// 注意这里的操作可能存在redis与数据库不同步的问题，可以使用消息队列解决，兜底方案为重新同步
	// 处理应该放在update中
	replyPost, err := uc.UpdatePost(ctx, &model.UpdatePostParam{
		Pid:  pid,
		Like: &newlike,
	})
	if err != nil {
		uc.log.Errorw(
			"[biz]", "AddPostLike/UpdatePost failed",
			"err", err,
			"pid", pid,
		)
	}
	return replyPost, nil
}

func GetUidFromCtx(ctx context.Context) (int64, error) {
	claimsToken, ok := jwtkratos.FromContext(ctx)
	if !ok {
		return -1, errTokenParase
	}
	claims, ok := claimsToken.(*Claims)
	if !ok {
		return -1, errToekenType
	}
	return claims.UID, nil
}

// 查后改锁
func (l *redisLocker) Lock(ctx context.Context, key string) (bool, error) {
	ok, err := l.Client.SetNX(ctx, key, l.id, l.ttl).Result()
	return ok, err
}

func (l *redisLocker) Unlock(ctx context.Context, key string) error {
	_, err := l.Client.Del(ctx, key).Result()
	return err
}
