package model

type LoginInfo struct {
	UID       int64  `db:"uid"`
	UserName  string `db:"user_name"`
	SecretPwd string `db:"password"`
}

type Response struct {
	Code int32
	Data *string
}
