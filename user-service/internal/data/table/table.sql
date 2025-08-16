CREATE TABLE `login_info` (
    `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
    `uid` BIGINT NOT NULL,
    `is_del` TINYINT NULL DEFAULT 0,
    `create_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `update_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `user_name` varchar(16) NOT NULL,
    `password` char(64) NOT NULL,
    INDEX `idx_user_name` (`user_name`),
    UNIQUE INDEX `idx_uid_type` (`uid`, `is_del`)
)
CREATE TABLE `user_info` (
    `id` BIGINT AUTO_INCREMENT,
    `uid` BIGINT NOT NULL,
    `is_del` TINYINT NULL DEFAULT 0,
    `create_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `update_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `user_name` varchar(16) NOT NULL,
    `birthday` DATE DEFAULT NULL,
    `phone` varchar(20) DEFAULT NULL,
    `gender` TINYINT DEFAULT 3, -- 3表示未知
    `points` BIGINT DEFAULT 0,
    `ext_json` varchar(256),
    UNIQUE INDEX `idx_uid_del` (`uid`, `is_del`),
    PRIMARY KEY (`id`)
)