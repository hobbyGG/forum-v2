package service

import (
	"context"

	pb "user-service/api/user/v1"
)

type UserService struct {
	pb.UnimplementedUserServer
}

func NewUserService() *UserService {
	return &UserService{}
}

func (s *UserService) GetUserByUID(ctx context.Context, req *pb.GetUserByUIDRequest) (*pb.GetUserByUIDReply, error) {
	return &pb.GetUserByUIDReply{}, nil
}
func (s *UserService) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.GetProfileReply, error) {
	return &pb.GetProfileReply{}, nil
}
