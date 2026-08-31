# Graduation Project 后端

## 一键本地运行

先安装并启动 Bocker 服务，然后直接使用 `Incusfile` 构建并运行：

```sh
bocker image build --name graduation-project ./Incusfile \
  && bocker image run graduation-project --name graduation-project
```

也可以将构建和运行拆成两条命令，便于重复启动已构建的镜像。

`autostart: true` 已写入 `Incusfile`。容器启动后由 Alpine/OpenRC 分别管理
PostgreSQL、Redis、LiveKit、API 和 Caddy；服务依赖和启动顺序写在
`configs/openrc/` 中。用户侧不需要额外的 up/down 包装脚本。
`autostart` 表示宿主机上的 Incus daemon 重启后自动启动该容器。

所有配置都直接写在仓库文件里，不依赖环境变量注入：数据库密码、JWT、OSS 和
LiveKit 凭据在 `configs/config.yaml`，LiveKit 服务端密钥在 `configs/livekit.yaml`，
TLS 证书路径固定写在 `configs/Caddyfile`（`/etc/caddy/certs/1.pem` 与 `1.key`）。
默认使用 `mini.gelsomino.cn` 域名和经 Caddy 代理的
`wss://mini.gelsomino.cn:444` LiveKit 信令地址，并启用公网 IP 探测。
`ssl/1.pem` 与 `ssl/1.key` 会被复制到镜像供 Caddy 使用；当前证书覆盖
`*.gelsomino.cn`。Bocker 会自动维护容器域名的 `/etc/hosts` 记录，因此本机与
服务器可以使用同一套静态配置进行部署和验收。

停止并清理容器：

```sh
bocker container remove graduation-project
```

完整黑盒验收覆盖 HTTP `81` 重定向、HTTPS `444`、两个 WebSocket 入口、所有业务
HTTP 路由以及 LiveKit 房间和视频轨道。
测试程序位于 `api/e2e_test.py`，测试用户和数据会在结束时清理。直接运行验收：

```sh
python3 -m venv .e2e-venv
.e2e-venv/bin/pip install -r api/requirements-e2e.txt
.e2e-venv/bin/python api/e2e_test.py --base https://graduation-project.test:444 \
  --http-base http://graduation-project.test:81 --insecure
```

远端使用自签名证书时，HTTP/WebSocket 可用 `--insecure`；LiveKit Python SDK
仍会校验证书，验收可把 LiveKit 的明文监听作为测试传输地址显式传入：

```sh
.e2e-venv/bin/python api/e2e_test.py --base https://mini.gelsomino.cn:444 \
  --http-base http://mini.gelsomino.cn:81 \
  --livekit-url ws://mini.gelsomino.cn:7880 --insecure
```

## Incusfile 构建说明

本项目的构建文件是根目录中的 `Incusfile`。文件没有扩展名，内容是 YAML。

`Incusfile` 使用两个阶段：

1. `builder` 阶段安装 Go，编译 API 和 LiveKit。
2. 最终阶段安装 PostgreSQL、Redis、Caddy 和 curl，只复制编译结果、配置文件与
   `configs/openrc/` 服务定义。OpenRC 分别启动各个服务；数据库初始化和 API 依赖
   等待由对应的 OpenRC service 完成，LiveKit 直接读取 `/etc/livekit/livekit.yaml`，
   Caddy 直接读取 `/etc/caddy/Caddyfile`。

部署结构保持单体：根目录只使用这一个 `Incusfile`。`configs/` 存放会被复制到
容器并由服务读取的静态 YAML、Caddy 配置和 OpenRC service 定义；`ssl/` 中的现有
证书会一并进入镜像。容器运行不依赖宿主机脚本。所有凭据均为明文硬编码，修改
`configs/` 下的文件后重新构建镜像即可生效。

```sh
bocker image build --name graduation-project ./Incusfile
bocker image run graduation-project --name graduation-project
```

## 生产部署

镜像内的配置全部来自仓库中硬编码的文件，适合本机一键验收；生产环境请直接修改
以下文件后重新构建，不要把真实凭据提交到公开仓库：

- `configs/config.yaml`：数据库、JWT、OSS、LiveKit 的全部凭据与地址。
- `configs/livekit.yaml`：LiveKit 服务端 `keys`，需与 `config.yaml` 中
  `livekit.api_key/api_secret` 保持一致；当前已启用 `rtc.use_external_ip`。
- `configs/openrc/graduation-db-init`：数据库初始化密码，需与 `config.yaml` 的
  `database.password` 保持一致。
- `configs/Caddyfile`：TLS 证书固定路径 `/etc/caddy/certs/1.pem` 与
  `/etc/caddy/certs/1.key`，生产证书直接替换 `ssl/` 下的文件。

数据目录建议通过 runtime mount 持久化，避免重建容器丢失状态：

```yaml
runtime:
  mounts:
    - {source: /srv/chat/data/postgresql, target: /var/lib/postgresql, mode: rw}
    - {source: /srv/chat/data/redis, target: /var/lib/redis, mode: rw}
    - {source: /srv/chat/data/livekit, target: /var/lib/livekit, mode: rw}
```

应用层仍保留了 `ZAT_*` 环境变量覆盖逻辑（见 `internal/config/config.go`），容器
内默认不设置任何环境变量，全部以配置文件为准。

详细的字段和步骤写法见 Bocker 仓库的 `README.md`。
