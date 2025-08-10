package data

import (
	"context"
	"post-service/internal/model"
)

// use mysql

func (repo *PostRepo) GetUserByUid(ctx context.Context, uid int64) (*model.User, error) {
	sqlStr := `
	select id, uid, is_del, user_name, create_at, update_at
	from login_info
	where uid = ? and is_del = 0`
	user := new(model.User)
	err := repo.data.MySqlCli.GetContext(ctx, user, sqlStr, uid)
	if err != nil {
		return nil, err
	}
	return user, nil
}
