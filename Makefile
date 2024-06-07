natpmp: main.go go.mod go.sum
	GOOS=linux GOARCH=amd64 GOAMD64=v3 go build -o natpmp -ldflags="-s -w" main.go

build: natpmp
