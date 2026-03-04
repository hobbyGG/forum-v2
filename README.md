# 通用社区服务系统

**技术栈**：Kratos, MySQL, PG, Redis, Kafka, consul, docker

简介：当前有评论服务和用户服务两个模块，都基于Kratos实现，Consul作为注册中心提供服务注册与服务发现功能。用户服务提供登录、注册，使用MySQL实现数据持久化。评论服务提供增删改查基本功能，使用PG实现数据持久化。
