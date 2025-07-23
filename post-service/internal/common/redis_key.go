package common

const (
	RKeyPostPrefix = "post:post_info:"         //post服务，post_info表
	RKeyPostList   = "post:post_list:%s:%d:%d" // post服务，post列表，列表类型，页数，页大小
	RKeyExpTime    = "post:post_info:expire"
	RKeyPostLike   = "post:post_like:%v" //记录每个post的点赞情况，存储pid与多个uid

	RKeyPostHotRank = "post:post_rank:hot"

	RKeyPostLock = "post:post_lock:%v" // post服务，post锁，key为post的唯一标识
)
