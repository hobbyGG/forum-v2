package data

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (repo *PostRepo) GetRankFromPG(ctx context.Context, limitNum ...int) map[int64]int64 {
	var sqlStr string
	var limit int
	var rows pgx.Rows
	var err error
	if len(limitNum) == 0 {
		sqlStr = `
			select pid, score
			from post_info
			where is_del = 0
			`
		rows, err = repo.data.PgxCli.Query(ctx, sqlStr)
	} else if len(limitNum) == 1 {
		limit = limitNum[0]
		sqlStr = `
			select pid, score
			from post_info
			where is_del = 0
			order by score desc
			limit $1`
		rows, err = repo.data.PgxCli.Query(ctx, sqlStr, limit)
	}

	if err != nil {
		repo.log.Errorw(
			"data", "GetRankFromPG/Query failed",
			"err", err,
		)
		panic(err)
	}
	defer rows.Close()

	hotMap := make(map[int64]int64, 100)
	for rows.Next() {
		var (
			pid   int64
			score int64
		)
		if err := rows.Scan(&pid, &score); err != nil {
			repo.log.Errorw(
				"data", "GetRankFromPG/Scan failed",
				"err", err,
			)
			panic(err)
		}
		hotMap[pid] = score
	}
	return hotMap
}
