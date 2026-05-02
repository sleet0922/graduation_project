.PHONY: run del

run:
	# 关闭8081进程
	-sudo fuser -k 8081/tcp 2>/dev/null || true
	> logs/app.log 2>/dev/null || true
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -extldflags=-static" -gcflags="all=-l=4 -B -C" -o ./main ./cmd/api/main.go
	chmod +x ./main
	nohup ./main >/dev/null 2>&1 &

del:
	rm -f ./main
	-sudo fuser -k 8081/tcp 2>/dev/null || true