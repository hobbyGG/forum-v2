package biz

import (
	"context"
	"errors"
	"time"
	"user-service/internal/common"
	"user-service/internal/conf"
	"user-service/internal/model"
	"user-service/third_party/encrypt"
	"user-service/third_party/jwt"

	"github.com/bwmarrin/snowflake"
	"github.com/go-kratos/kratos/v2/log"
)

type AuthRepo interface {
	GetLogInfoByUserName(ctx context.Context, username string) (*model.LoginInfo, error)

	SaveLoginInfo(ctx context.Context, loginInfo *model.LoginInfo) error
}

type AuthUsecase struct {
	repo AuthRepo
	log  *log.Helper

	pwdSecrete []byte
	jwtSecrete []byte

	sfNode *snowflake.Node
}

func NewAuthUsecase(repo AuthRepo, c *conf.Biz, sfNode *snowflake.Node, logger log.Logger) *AuthUsecase {
	return &AuthUsecase{
		repo:       repo,
		log:        log.NewHelper(logger),
		pwdSecrete: []byte(c.Auth.PwdSecrete),
		jwtSecrete: []byte(c.Auth.JwtSecrete),
		sfNode:     sfNode,
	}
}

func NewSfNode(c *conf.Biz) *snowflake.Node {
	if c == nil {
		panic(errors.New("conf.biz is nil"))
	}
	epoch, err := time.Parse("2006-01-02 15:04:05", c.App.StartTime)
	if err != nil {
		panic(err)
	}
	snowflake.Epoch = epoch.UnixMilli()

	node, err := snowflake.NewNode(c.App.MachineID)
	if err != nil {
		panic(err)
	}
	if node == nil {
		panic("sfnode is nil")
	}
	return node
}

func (uc *AuthUsecase) Login(ctx context.Context, param *model.LoginParam) (*model.Response, error) {
	// 密码检查
	loginInfo, err := uc.repo.GetLogInfoByUserName(ctx, param.UserName)
	if err != nil {
		if !errors.Is(err, common.ErrSQLNotFound) {
			return &model.Response{
				Code: common.CodeInternalErr,
				Data: nil,
			}, err
		}
		// 没找到对应用户
		return &model.Response{
			Code: common.CodeUserNotFond,
			Data: nil,
		}, common.ErrUserNotExist
	}
	if encrypt.SHA256([]byte(param.Pwd), uc.pwdSecrete) != loginInfo.SecretPwd {

		// 密码错误
		return &model.Response{
			Code: common.CodePwdErr,
			Data: nil,
		}, common.ErrLoginFail
	}

	// 登录成功,获取JWT
	token, err := jwt.New(uc.jwtSecrete, jwt.WithUID(loginInfo.UID))
	if err != nil {
		return &model.Response{
			Code: common.CodeInternalErr,
			Data: nil,
		}, err
	}

	return &model.Response{
		Code: common.CodeSuccess,
		Data: &token,
	}, nil
}

func (uc *AuthUsecase) SignUp(ctx context.Context, signupParam *model.SignupParam) (*model.Response, error) {
	if uc.sfNode == nil {
		uc.log.Debugw(
			"[biz]", "auth.go",
			"SignUp error", errors.New("sfnode is nil"),
		)
		return &model.Response{
			Code: common.CodeInternalErr,
			Data: nil,
		}, common.ErrInternal
	}
	if signupParam.Pwd != signupParam.RePwd {
		return &model.Response{
			Code: common.CodePwdErr,
			Data: nil,
		}, common.ErrPwdNotConsist
	}

	secretPwd := encrypt.SHA256([]byte(signupParam.Pwd), uc.pwdSecrete)
	uid := uc.sfNode.Generate().Int64()
	info := &model.LoginInfo{
		UID:       uid,
		UserName:  signupParam.UserName,
		SecretPwd: secretPwd,
	}
	if err := uc.repo.SaveLoginInfo(ctx, info); err != nil {
		return &model.Response{
			Code: common.CodeInternalErr,
			Data: nil,
		}, err
	}

	return &model.Response{
		Code: common.CodeSuccess,
		Data: nil,
	}, nil
}
