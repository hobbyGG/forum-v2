package service

import (
	"context"
	"errors"
	"post-service/internal/model"

	pb "post-service/api/post/v1"
	"post-service/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PostSrvService struct {
	pb.UnimplementedPostSrvServer

	uc  *biz.PostUsecase
	log *log.Helper
}

func NewPostSrvService(uc *biz.PostUsecase, logger log.Logger) *PostSrvService {
	return &PostSrvService{uc: uc, log: log.NewHelper(logger)}
}

var (
	errPermissionDenied = errors.New("permission denied")
)

func (s *PostSrvService) CreatePost(ctx context.Context, req *pb.CreatePostRequest) (*pb.CreatePostReply, error) {
	// 请求处理
	if req.Tags == nil {
		req.Tags = []string{"default"}
	}
	param := model.CreatePostParam{
		Title:   req.Title,
		Content: req.Content,
		Tags:    req.Tags,
	}

	// 调用 Usecase 层的业务逻辑
	post, err := s.uc.CreatePost(ctx, &param)
	if err != nil {
		s.log.Errorw(
			"[service]", "CreatePost",
			"err", err,
		)
		return nil, err
	}

	respPost := pb.Post{
		Id:         post.Id,
		Pid:        post.Pid,
		IsDel:      post.IsDel,
		CreateTime: timestamppb.New(post.CreateTime),
		UpdateTime: timestamppb.New(post.UpdateTime),

		Title:     post.Title,
		Content:   post.Content,
		Author:    post.Author,
		Uid:       post.Uid,
		Status:    post.Status,
		Score:     post.Score,
		Tags:      post.Tags,
		ViewCount: post.View,
		LikeCount: post.Like,
	}
	return &pb.CreatePostReply{
		Code: 200,
		Post: &respPost,
	}, nil
}
func (s *PostSrvService) UpdatePost(ctx context.Context, req *pb.UpdatePostRequest) (*pb.UpdatePostReply, error) {
	// 请求处理
	if req.Status != nil {
		if req.Type != 1 {
			return nil, errPermissionDenied
		}
	}

	param := model.UpdatePostParam{
		Pid:     req.Pid,
		Title:   req.Title,
		Content: req.Content,
		Status:  req.Status,
		Score:   req.Score,
		Tags:    req.Tags,
	}

	// 调用 Usecase 层的业务逻辑
	post, err := s.uc.UpdatePost(ctx, &param)
	if err != nil {
		s.log.Errorw(
			"[service]", "UpdatePost",
			"err", err,
		)
		return nil, err
	}
	respPost := pb.Post{
		Id:         post.Id,
		Pid:        post.Pid,
		IsDel:      post.IsDel,
		CreateTime: timestamppb.New(post.CreateTime),
		UpdateTime: timestamppb.New(post.UpdateTime),
		Title:      post.Title,
		Content:    post.Content,
		Author:     post.Author,
		Uid:        post.Uid,
		Status:     post.Status,
		Score:      post.Score,
		Tags:       post.Tags,
		ViewCount:  post.View,
		LikeCount:  post.Like,
	}

	return &pb.UpdatePostReply{
		Code: 200,
		Post: &respPost,
	}, nil
}
func (s *PostSrvService) DeletePost(ctx context.Context, req *pb.DeletePostRequest) (*pb.DeletePostReply, error) {
	pid := req.Pid
	if err := s.uc.DeletePost(ctx, pid); err != nil {
		s.log.Errorw(
			"[service]", "DeletePost",
			"err", err,
		)
		return nil, err
	}
	return &pb.DeletePostReply{
		Code: 200,
	}, nil
}

func (s *PostSrvService) GetPostPreview(ctx context.Context, req *pb.GetPostPreviewRequest) (*pb.GetPostPreviewReply, error) {
	return nil, errors.New("该接口未实现")
}

func (s *PostSrvService) GetPostDetail(ctx context.Context, req *pb.GetPostDetailRequest) (*pb.GetPostDetailReply, error) {
	pid := req.Pid
	post, err := s.uc.GetPostById(ctx, pid)
	if err != nil {
		s.log.Errorw(
			"[service]", "GetPostDetail",
			"err", err,
		)
		return nil, err
	}
	return &pb.GetPostDetailReply{
		Code: 200,
		Post: &pb.Post{
			Id:         post.Id,
			Pid:        post.Pid,
			IsDel:      post.IsDel,
			CreateTime: timestamppb.New(post.CreateTime),
			UpdateTime: timestamppb.New(post.UpdateTime),
			Title:      post.Title,
			Content:    post.Content,
			Author:     post.Author,
			Uid:        post.Uid,
			Status:     post.Status,
			Score:      post.Score,
			Tags:       post.Tags,
			ViewCount:  post.View,
			LikeCount:  post.Like,
		},
	}, nil
}
func (s *PostSrvService) ListPostPreview(ctx context.Context, req *pb.ListPostPreviewRequest) (*pb.ListPostPreviewReply, error) {
	page := req.Page
	size := req.PageSize
	searchType := req.Type

	posts, err := s.uc.ListPostPreview(ctx, page, size, searchType)
	if err != nil {
		s.log.Errorw(
			"[service]", "ListPost",
			"err", err,
		)
		return nil, err
	}

	respPosts := make([]*pb.PostPreview, 0, len(posts))
	for _, post := range posts {
		respPosts = append(respPosts, &pb.PostPreview{
			Id:         post.Id,
			Pid:        post.Pid,
			CreateTime: timestamppb.New(post.CreateTime),
			UpdateTime: timestamppb.New(post.UpdateTime),
			Title:      post.Title,
			Content:    post.Content,
			Author:     post.Author,
			Status:     post.Status,
			Score:      post.Score,
			Tags:       post.Tags,
			ViewCount:  post.ViewCount,
			LikeCount:  post.LikeCount,
		})
	}

	return &pb.ListPostPreviewReply{
		Code:  200,
		Posts: respPosts,
	}, nil
}

func (s *PostSrvService) AddPostLike(ctx context.Context, req *pb.AddPostLikeRequest) (*pb.AddPostLikeReply, error) {
	post, err := s.uc.AddPostLike(ctx, req.Pid, req.Like)
	if err != nil {
		s.log.Errorw(
			"[service]", "AddPostLike",
			"err", err,
		)
		return nil, err
	}
	respPost := pb.Post{
		Id:         post.Id,
		Pid:        post.Pid,
		IsDel:      post.IsDel,
		CreateTime: timestamppb.New(post.CreateTime),
		UpdateTime: timestamppb.New(post.UpdateTime),
		Title:      post.Title,
		Content:    post.Content,
		Author:     post.Author,
		Uid:        post.Uid,
		Status:     post.Status,
		Score:      post.Score,
		Tags:       post.Tags,
		ViewCount:  post.View,
		LikeCount:  post.Like,
	}
	return &pb.AddPostLikeReply{
		Code: 200,
		Post: &respPost,
	}, nil
}
