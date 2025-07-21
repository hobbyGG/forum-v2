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