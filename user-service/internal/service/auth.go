package service

import (
	"context"

	pb "user-service/api/auth/v1"
	"user-service/internal/biz"
	"user-service/internal/model"
)

type AuthService struct {
	pb.UnimplementedAuthServer

	uc *biz.AuthUsecase
}

func NewAuthService(uc *biz.AuthUsecase) *AuthService {
	return &AuthService{uc: uc}
}

func (s *AuthService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	// 请求参数处理
	param := &model.LoginParam{
		UserName: req.UserName,
		Pwd:      req.Password,
	}

	res, err := s.uc.Login(ctx, param)
	if err != nil {
		return &pb.LoginResponse{
			Code: res.Code,
		}, err
	}

	return &pb.LoginResponse{
		Code: res.Code,
		Data: *res.Data,
	}, nil
}
func (s *AuthService) Signup(ctx context.Context, req *pb.SignupRequest) (*pb.SignupResponse, error) {
	signupParam := &model.SignupParam{
		UserName: req.UserName,
		Pwd:      req.Password,
		RePwd:    req.RePassword,
		Tel:      req.Tel,
	}
	res, err := s.uc.SignUp(ctx, signupParam)
	if err != nil {
		return &pb.SignupResponse{
			Code: res.Code,
		}, err
	}
	return &pb.SignupResponse{
		Code: res.Code,
	}, nil
}
