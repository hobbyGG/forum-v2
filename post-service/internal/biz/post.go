package biz

import (
	"context"
	"database/sql"
	"errors"
	"post-service/internal/conf"
	"post-service/internal/model"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/go-kratos/kratos/v2/log"
	jwtkratos "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/golang-jwt/jwt/v5"
)

type PostRepo interface {
	CreatePost(ctx context.Context, post *model.CreatePostParam) (*model.Post, error)
	UpdatePost(ctx context.Context, post *model.UpdatePostParam) (*model.Post, error)
	DeletePost(ctx context.Context, id int64) error
	GetPostById(ctx context.Context, id int64) (*model.Post, error)
	ListPostPreview(ctx context.Context, page, pageSize int64) ([]*model.PostPreview, error)

	GetUserByUid(ctx context.Context, uid int64) (*model.User, error)
}

type PostUsecase struct {
	repo PostRepo
	node *snowflake.Node
	log  log.Helper
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

	return &PostUsecase{
		repo: repo,
		node: node,
		log:  *log.NewHelper(logger),
	}
}

type Claims struct {
	UID int64 `json:"uid"`
	jwt.RegisteredClaims
}

var (
	errTokenParase = errors.New("token is invalid")
	errToekenType  = errors.New("token type is invalid")

	errPostNotExisted = errors.New("post not existed")
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

func (uc *PostUsecase) UpdatePost(ctx context.Context, param *model.UpdatePostParam) (*model.Post, error) {
	// 检查该post是否存在
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

	uc.log.WithContext(ctx).Infof("Updating post with PID: %d", param.Pid)
	post, err := uc.repo.UpdatePost(ctx, param)
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

func (uc *PostUsecase) ListPostPreview(ctx context.Context, page, pageSize int64) ([]*model.PostPreview, error) {
	uc.log.WithContext(ctx).Infof("Listing posts with page: %d, pageSize: %d", page, pageSize)
	posts, err := uc.repo.ListPostPreview(ctx, page, pageSize)
	if err != nil {
		uc.log.Errorw(
			"[biz]", "ListPostPreview/ListPostPreview failed",
			"err", err,
		)
		return nil, err
	}
	return posts, nil
}
