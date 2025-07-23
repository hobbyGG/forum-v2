package model

import "time"

type Post struct {
	Id         int64     `json:"id" db:"id"`
	Pid        int64     `json:"pid" db:"pid"`
	IsDel      *int32    `json:"is_del" db:"is_del"`
	CreateTime time.Time `json:"create_time" db:"create_time"`
	UpdateTime time.Time `json:"update_time" db:"update_time"`
	Title      string    `json:"title" db:"title"`
	Content    string    `json:"content" db:"content"`
	Author     string    `json:"author" db:"author"`
	Uid        int64     `json:"uid" db:"uid"`
	Status     int32     `json:"status" db:"status"`
	Score      int64     `json:"score" db:"score"`
	Tags       []string  `json:"tags" db:"tags"`
	View       int64     `json:"view" db:"view"`
	Like       int64     `json:"like" db:"like"`
}

type PostPreview struct {
	Id         int64     `json:"id" db:"id"`
	Pid        int64     `json:"pid" db:"pid"`
	CreateTime time.Time `json:"create_time" db:"create_time"`
	UpdateTime time.Time `json:"update_time" db:"update_time"`
	Title      string    `json:"title" db:"title"`
	Content    string    `json:"content" db:"content"`
	Author     string    `json:"author" db:"author"`
	Status     int32     `json:"status" db:"status"`
	Score      int64     `json:"score" db:"score"`
	Tags       []string  `json:"tags" db:"tags"`
	ViewCount  int64     `json:"view_count" db:"view"`
	LikeCount  int64     `json:"like_count" db:"like"`
}

func (p *Post) ScanArgs() []any {
	return []any{
		&p.Id, &p.Pid, &p.IsDel, &p.CreateTime, &p.UpdateTime,
		&p.Title, &p.Content, &p.Author, &p.Uid, &p.Status,
		&p.Score, &p.Tags, &p.View, &p.Like,
	}
}

func (p *Post) ToPreview() *PostPreview {
	return &PostPreview{
		Id:         p.Id,
		Pid:        p.Pid,
		CreateTime: p.CreateTime,
		UpdateTime: p.UpdateTime,
		Title:      p.Title,
		Content:    p.Content,
		Author:     p.Author,
		Status:     p.Status,
		Score:      p.Score,
		Tags:       p.Tags,
		ViewCount:  p.View,
		LikeCount:  p.Like,
	}
}

func (p *PostPreview) ScanArgs() []any {
	return []any{
		&p.Id, &p.Pid, &p.CreateTime, &p.UpdateTime,
		&p.Title, &p.Content, &p.Author, &p.Status,
		&p.Score, &p.Tags, &p.ViewCount, &p.LikeCount,
	}
}
