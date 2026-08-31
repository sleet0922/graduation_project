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

默认使用 `graduation-project.test` 和 `ws://...:7880` 的 LiveKit 信令地址，并关闭
需要公网 STUN 的外部 IP 探测以适配 NAT。`ssl/1.pem` 与 `ssl/1.key` 会被复制到
镜像供 Caddy 使用；当前证书覆盖 `*.gelsomino.cn`，部署该域名时需通过 build arg
覆盖默认域名。Bocker 会自动维护容器域名的 `/etc/hosts` 记录。

停止并清理容器：

```sh
bocker container remove graduation-project
```

在具备可达公网 STUN 的生产网络中，可在构建时传入
`--build-arg LIVEKIT_USE_EXTERNAL_IP=true`。完整黑盒验收覆盖 HTTP `81` 重定向、
HTTPS `444`、两个 WebSocket 入口、所有业务 HTTP 路由以及 LiveKit 房间和视频轨道。
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
   `configs/openrc/` 服务定义。OpenRC 分别启动各个服务；数据库初始化、LiveKit
   运行配置、API 依赖等待和 Caddy TLS 准备均由对应的 OpenRC service 完成。

部署结构保持单体：根目录只使用这一个 `Incusfile`。`configs/` 存放会被复制到
容器并由服务读取的静态 YAML、Caddy 配置和 OpenRC service 定义；`ssl/` 中的现有
证书会一并进入镜像。容器运行不依赖宿主机脚本。生产环境应在证书到期或泄露时
立即替换证书和私钥，并优先改用下述只读 runtime mount。

直接构建镜像时可覆盖构建参数：

```sh
bocker image build --name graduation-project \
  --build-arg APP_DOMAIN=graduation-project.test \
  --build-arg LIVEKIT_URL=ws://graduation-project.test:7880 \
  --build-arg TLS_MODE=local ./Incusfile
bocker image run graduation-project --name graduation-project
```

## 生产部署契约

镜像默认是 `development` 模式，只适合本机一次性验收。生产启动必须同时设置
`BOCKER_DEPLOYMENT_MODE=production` 和 `BOCKER_TLS_MODE=production`。Bocker 当前
没有 `image run --env` 参数，因此生产密钥不要写进镜像构建参数或共享镜像；请由
部署系统生成权限为 `0600` 的 `/etc/bocker.env`，并在 `runtime.mounts` 中以只读方式挂载。
Bocker 会把运行环境写入 `/etc/bocker.env`，OpenRC service 启动时读取该文件。
生产服务会拒绝空值、占位符和本地开发默认值；LiveKit API key/secret 仅接受字母、
数字、`.`、`_`、`-`。

生产证书必须以只读 runtime mount 挂载到容器，例如：

```yaml
runtime:
  env:
    BOCKER_DEPLOYMENT_MODE: production
    BOCKER_TLS_MODE: production
    BOCKER_TLS_CERT: /etc/caddy/certs/server.pem
    BOCKER_TLS_KEY: /etc/caddy/certs/server.key
  mounts:
    - {source: /srv/chat/secrets/bocker.env, target: /etc/bocker.env, mode: ro}
    - {source: /srv/chat/tls/server.pem, target: /etc/caddy/certs/server.pem, mode: ro}
    - {source: /srv/chat/tls/server.key, target: /etc/caddy/certs/server.key, mode: ro}
    # 按需将以下目录挂载到宿主机，避免重建容器丢失状态：
    - {source: /srv/chat/data/postgresql, target: /var/lib/postgresql, mode: rw}
    - {source: /srv/chat/data/redis, target: /var/lib/redis, mode: rw}
    - {source: /srv/chat/data/livekit, target: /var/lib/livekit, mode: rw}
```

不要把上面的真实值写入版本库；建议由部署系统渲染 runtime 配置和
`/etc/bocker.env`。该文件包含 `ZAT_DATABASE_PASSWORD`、`ZAT_JWT_SECRET`、
`ZAT_OSS_ACCESS_KEY_ID`、`ZAT_OSS_SECRET_ACCESS_KEY`、`ZAT_LIVEKIT_URL`、
`ZAT_LIVEKIT_API_KEY` 和 `ZAT_LIVEKIT_API_SECRET`。生产模式会根据这些值在 `/run`
生成 LiveKit 服务端配置，静态模板保持无凭据。

详细的字段和步骤写法见 Bocker 仓库的 `README.md`。
