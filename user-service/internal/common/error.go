package common

import (
	"database/sql"
	"errors"
)

var (
	// sql错误
	ErrSQLNotFound = sql.ErrNoRows

	// redis错误

	// 服务错误
	ErrInternal     = errors.New("内部错误")
	ErrUserNotExist = errors.New("用户不存在")
	ErrLoginFail    = errors.New("用户名或密码错误")
	ErrGetToken     = errors.New("获取token错误")

	ErrPwdNotConsist = errors.New("两次密码不一致")
)
