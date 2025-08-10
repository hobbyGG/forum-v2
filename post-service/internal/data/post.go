package data

import (
	"context"
	"errors"
	"post-service/internal/biz"
	"post-service/internal/model"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
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
	sfg          = singleflight.Group{}
	redisBreaker = gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "RedisBreaker"})
	pgBreaker    = gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "PGBreaker"})
)

type PostRepo struct {
	data *Data
	log  *log.Helper
}

func NewPostRepo(data *Data, logger log.Logger) biz.PostRepo {
	// 初始化时进行缓存预热
	repo := &PostRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
	repo.WarmUpPostCache(context.Background())
	return repo
}

func (repo *PostRepo) RedisClient() *redis.Client {
	return repo.data.Rcli
}

func NewPostRepoForJob(data *Data, logger log.Logger) *PostRepo {
	return &PostRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (repo *PostRepo) CreatePost(ctx context.Context, post *model.CreatePostParam) (*model.Post, error) {
	sqlStr := `
	insert into post_info(pid, title, content, author, uid, score, tags)
	values($1, $2, $3, $4, $5, $6, $7)
	returning id, pid, is_del, create_time, update_time, title, content, author, uid, status, score, tags, view, "like";`
	replyPost := new(model.Post)

	res, err := pgBreaker.Execute(func() (interface{}, error) {
		if err := repo.data.PgxCli.QueryRow(ctx, sqlStr, post.ToArgs()...).
			Scan(replyPost.ScanArgs()...); err != nil {
			repo.log.Errorw(
				"[repo]", "CreatePost/QueryRow failed",
				"err", err,
				"postParam", post,
			)
			return nil, err
		}
		return replyPost, nil
	})
	if err != nil {
		repo.log.Errorw(
			"[repo]", "CreatePost/Execute failed",
			"err", err,
		)
		return nil, err
	}

	// 将新创建的帖子加入排行榜
	if err := repo.AddHotRank(ctx, replyPost.Pid, replyPost.Score); err != nil {
		repo.log.Errorw(
			"[repo]", "CreatePost/AddHotRank failed",
			"pid", replyPost.Pid,
		)
		return nil, err
	}
	if err := delCacheAfterWrite(ctx, repo, post.Pid); err != nil {
		repo.log.Errorw(
			"[repo]", "CreatePost/DelPostFC failed",
			"pid", post.Pid)
	}
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

	res, err := pgBreaker.Execute(func() (interface{}, error) {
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

	replyPost := res.(*model.Post)
	// 更新成功后将帖子加入排行榜
	if err := repo.AddHotRank(ctx, replyPost.Pid, replyPost.Score); err != nil {
		repo.log.Errorw(
			"[repo]", "UpdatePost/AddHotRank failed",
			"pid", replyPost.Pid,
		)
		return nil, err
	}
	// 写删除
	if err := delCacheAfterWrite(ctx, repo, replyPost.Pid); err != nil {
		repo.log.Errorw(
			"[repo]", "UpdatePost/DelPostFC failed",
			"err", err,
			"pid", replyPost.Pid,
		)
	}
	return replyPost, nil
}

func (repo *PostRepo) DeletePost(ctx context.Context, id int64) error {
	// 实现逻辑删除
	sqlStr := `
	update post_info
	set is_del = null
	where pid = $1`

	_, err := pgBreaker.Execute(func() (interface{}, error) {
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

	if err := repo.DelHotRankMem(ctx, id); err != nil {
		repo.log.Errorw(
			"[repo]", "DeletePost/DelHotRank failed",
			"pid", id,
		)
	}
	if err := delCacheAfterWrite(ctx, repo, id); err != nil {
		repo.log.Errorw(
			"[repo]", "DeletePost/DelPostFC failed",
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

func (repo *PostRepo) ListPostPreviewByTime(ctx context.Context, page, pageSize int64) ([]*model.PostPreview, error) {
	sqlStr := `
	select id, pid, create_time, update_time, title, content, author, status, score, tags, view, "like"
	from post_info 
	where is_del = 0
	order by update_time desc
	limit $1 offset $2`
	return repo.ListPostPreview(ctx, page, pageSize, sqlStr)
}

func (repo *PostRepo) ListPostPreviewByHotFallback(ctx context.Context, page, pageSize int64) ([]*model.PostPreview, error) {
	sqlStr := `
	select id, pid, create_time, update_time, title, content, author, status, score, tags, view, "like"
	from post_info 
	where is_del = 0
	order by score desc, view desc
	limit $1 offset $2`
	return repo.ListPostPreview(ctx, page, pageSize, sqlStr)
}

func (repo *PostRepo) ListPostPreview(ctx context.Context, page, pageSize int64, sqlStr string) ([]*model.PostPreview, error) {
	// 可选：增加列表缓存，已经定义的参数(key)
	res, err := pgBreaker.Execute(func() (interface{}, error) {
		rows, err := repo.data.PgxCli.Query(ctx, sqlStr, pageSize, page*pageSize)
		return rows, err
	})
	if err != nil {
		repo.log.Errorw(
			"[repo]", "ListPosts/Query failed",
			"err", err,
		)
		return nil, err
	}
	rows := res.(pgx.Rows)
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
