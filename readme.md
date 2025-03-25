# 说明
~~(没发Release版本就是不能用)~~

一个将对话式接口转换为OpenAI标准格式的Gin中间件

# 返回格式


# 特性
1. 无需每次都新建新对话，在对话未修改时可以继续使用原对话
2. 使用BBolt（内置）/Redis（外置但可设置对话过期时间）存储对话MD5缓存，占用低

# 自编译
go build conversation2api -tags "redis bbolt" -ldflags '-s -w'

