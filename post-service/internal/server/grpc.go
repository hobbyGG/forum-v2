package server

import (
	v1 "post-service/api/post/v1"
	"post-service/internal/biz"
	"post-service/internal/conf"
	"post-service/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	kratosjwt "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/golang-jwt/jwt/v5"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server, poster *service.PostSrvService, logger log.Logger) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
			validate.Validator(),

			kratosjwt.Server(func(token *jwt.Token) (interface{}, error) {
				return []byte(c.JwtSecret), nil
			}, kratosjwt.WithClaims(func() jwt.Claims {
				return &biz.Claims{}
			}), //在这里增加jwt的option
			),
		),
	}
	if c.Grpc.Network != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	v1.RegisterPostSrvServer(srv, poster)
	return srv
}
