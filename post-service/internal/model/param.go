package model

import "github.com/lib/pq"

type CreatePostParam struct {
	Pid     int64    `json:"pid" db:"pid"`
	Title   string   `json:"title" db:"title"`
	Content string   `json:"content" db:"content"`
	Author  string   `json:"author" db:"author"`
	Uid     int64    `json:"uid" db:"uid"`
	Score   int64    `json:"score" db:"score"`
	Tags    []string `json:"tags" db:"tags"`
}
type UpdatePostParam struct {
	Pid     int64    `json:"pid" db:"pid"`
	Title   *string  `json:"title" db:"title"`
	Content *string  `json:"content" db:"content"`
	Status  *int32   `json:"status" db:"status"`
	Score   *int64   `json:"score" db:"score"`
	Tags    []string `json:"tags" db:"tags"`
}

func (p *CreatePostParam) ToArgs() []interface{} {
	return []interface{}{
		p.Pid, p.Title, p.Content, p.Author, p.Uid, p.Score, pq.Array(p.Tags),
	}
}
func (p *UpdatePostParam) ToArgs() []interface{} {
	return []interface{}{
		p.Pid, p.Title, p.Content, p.Status, p.Score, p.Tags,
	}
}
