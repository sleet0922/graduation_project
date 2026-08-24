# 本项目的 Incusfile

本项目的构建文件是根目录中的 `Incusfile`。文件没有扩展名，内容是 YAML。

`Incusfile` 使用两个阶段：

1. `builder` 阶段安装 Go，编译 API 和 LiveKit。
2. 最终阶段安装 PostgreSQL、Redis 和 Caddy，只复制编译结果与配置文件。

部署结构保持单体：根目录只使用这一个 `Incusfile`。`configs/` 存放会被复制到
容器并由服务读取的静态配置，包括 YAML、systemd unit、Caddy 配置和证书；只有
需要独立执行的初始化或运维程序才放入 `scripts/`，当前项目没有额外脚本目录。

详细的字段和步骤写法见 Bocker 仓库的 `README.md`。
