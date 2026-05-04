.PHONY: run del

run:
	-sudo fuser -k 8081/tcp 2>/dev/null || true
	> logs/app.log 2>/dev/null || true
	CGO_ENABLED=0 GOAMD64=v3 go build -trimpath -ldflags="-s -w -extldflags=-static -compressdwarf=false" -gcflags="all=-l=4 -B -C -d=checkptr=0" -o ./main ./cmd/api/main.go
	chmod +x ./main
	nohup ./main >/dev/null 2>&1 &

del:
	rm -f ./main
	-sudo fuser -k 8081/tcp 2>/dev/null || true