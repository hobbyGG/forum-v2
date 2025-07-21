-- CREATE TABLE `post_info` (
--     `id` BIGINT NOT NULL PRIMARY KEY AUTO_INCREMENT,
--     `pid` BIGINT NOT NULL,
--     `is_del` TINYINT NULL DEFAULT 0, -- mysql允许在unique下存储多个null值
--     `create_time` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
--     `update_time` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

--     `title` varchar(64) NOT NULL,
--     `content` text NOT NULL,
--     `author` varchar(16) NOT NULL,
--     `uid` BIGINT NOT NULL,
--     `status` TINYINT NOT NULL DEFAULT 0, -- 0正常，1待审核，2隐藏，4置顶
--     `score` BIGINT NOT NULL, -- 热度 以ms为单位
--     `tags` VARCHAR(255) NULL, -- 可以用逗号分割
--     `view` BIGINT NOT NULL DEFAULT 0,
--     `like` BIGINT NOT NULL DEFAULT 0
-- )

CREATE TABLE post_info (
    id bigserial not null,
    pid bigint NOT NULL,
    is_del smallint DEFAULT 0,
    create_time timestamp NOT NULL DEFAULT current_timestamp,
    update_time timestamp NOT NULL DEFAULT current_timestamp,

    title varchar(64) NOT NULL,
    content text NOT NULL,
    author varchar(16) NOT NULL,
    uid bigint NOT NULL,
    status smallint NOT NULL DEFAULT 0, -- 0待审核，1正常，2隐藏，4置顶
    score bigint NOT NULL, -- 热度 以ms为单位
    tags varchar(64)[] NOT NULL,
    view bigint NOT NULL DEFAULT 0,
    "like" bigint NOT NULL DEFAULT 0,

    primary key (id),
    unique (pid, is_del)
);