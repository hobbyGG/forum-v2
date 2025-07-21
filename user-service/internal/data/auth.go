package data

import (
	"context"
	"user-service/internal/biz"
	"user-service/internal/model"

	"github.com/go-kratos/kratos/v2/log"
)

type authRepo struct {
	data *Data
	log  *log.Helper
}

func NewAuthRepo(data *Data, logger log.Logger) biz.AuthRepo {
	return &authRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (repo *authRepo) GetLogInfoByUserName(ctx context.Context, userNmae string) (*model.LoginInfo, error) {
	sqlStr := `
	select uid, user_name, password from login_info
	where user_name = ?`
	info := new(model.LoginInfo)
	if err := repo.data.db.GetContext(ctx, info, sqlStr, userNmae); err != nil {
		repo.log.Debugw(
			"[data]", "auth.go",
			"GetLogInfoByUserName error", err)
		return nil, err
	}

	return info, nil
}

func (repo *authRepo) SaveLoginInfo(ctx context.Context, loginInfo *model.LoginInfo) error {
	sqlStr := `
	insert into login_info(uid, user_name, password) 
	value(?, ?, ?)`
	_, err := repo.data.db.ExecContext(ctx, sqlStr, loginInfo.UID, loginInfo.UserName, loginInfo.SecretPwd)
	if err != nil {
		repo.log.Debugw(
			"[data]", "auth.go",
			"SaveLoginInfo error", err)
		return err
	}
	return nil
}
