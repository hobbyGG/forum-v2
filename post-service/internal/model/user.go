package model

import "time"

type User struct {
	Id       int64     `json:"id" db:"id"`
	Uid      int64     `json:"uid" db:"uid"`
	IsDel    *int32    `json:"is_del" db:"is_del"`
	CreateAt time.Time `json:"create_at" db:"create_at"`
	UpdateAt time.Time `json:"update_at" db:"update_at"`
	Username string    `json:"user_name" db:"user_name"`
}
