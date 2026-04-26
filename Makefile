.PHONY: run

run:
	# 关闭8081进程
	-sudo fuser -k 8081/tcp 2>/dev/null || true

	# 清空日志
	> logs/app.log 2>/dev/null || true

	# 编译Go
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -extldflags=-static" -gcflags="all=-l=4 -B -C" -o ./main ./cmd/api/main.go

	# 授权
	chmod +x ./main

	# 后台启动
	nohup ./main >/dev/null 2>&1 &