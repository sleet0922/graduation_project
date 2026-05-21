.PHONY: run del docker docker-stop docker-logs docker-clean docker-shell docker-psql docker-redis

run:
	-sudo fuser -k 8081/tcp 2>/dev/null || true
	> logs/app.log 2>/dev/null || true
	CGO_ENABLED=0 GOAMD64=v3 go build -trimpath -ldflags="-s -w -extldflags=-static -compressdwarf=false" -gcflags="all=-l=4 -B -C -d=checkptr=0" -o ./main ./cmd/api/main.go
	chmod +x ./main
	nohup ./main >/dev/null 2>&1 &

del:
	rm -f ./main
	-sudo fuser -k 8081/tcp 2>/dev/null || true

docker:
	@cp configs/config.yaml configs/config.yaml.bak
	@sed -i 's/192.168.3.114/127.0.0.1/g' configs/config.yaml
	@echo "编译 Go 程序..."
	@CGO_ENABLED=0 GOAMD64=v3 go build -trimpath -ldflags="-s -w -extldflags=-static -compressdwarf=false" -gcflags="all=-l=4 -B -C -d=checkptr=0" -o ./main ./cmd/api/main.go
	@chmod +x ./main
	@echo "下载 Alpine minirootfs..."
	@wget -q https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/x86_64/alpine-minirootfs-3.21.3-x86_64.tar.gz
	@echo "构建 Docker 镜像..."
	@docker build -t graduation-app:latest .
	@echo "删除 Alpine minirootfs..."
	@rm -f alpine-minirootfs-3.21.3-x86_64.tar.gz
	@-docker stop graduation-app 2>/dev/null || true
	@-docker rm graduation-app 2>/dev/null || true
	@echo "启动 Docker 容器..."
	@docker run -d \
 		--name graduation-app \
 		-p 0.0.0.0:5432:5432 \
 		-p 0.0.0.0:6379:6379 \
 		-p 0.0.0.0:8081:8081 \
		-v postgres_data:/var/lib/postgresql/data \
 		-v redis_data:/data \
 		graduation-app:latest
	@mv configs/config.yaml.bak configs/config.yaml
	@echo "删除本地 main 文件..."
	@rm -f ./main
	@echo "完成！"
	@echo "查看日志: make docker-logs"

docker-stop:
	@-docker stop graduation-app
	@-docker rm graduation-app

docker-logs:
	@docker logs -f graduation-app

docker-shell:
	@docker exec -it graduation-app /bin/bash
