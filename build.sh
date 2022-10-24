VERSION=`date -u +%Y%m%d`
LDFLAGS="-X main.VERSION=$VERSION -s -w"
GCFLAGS=""

env CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o koiAliddns_mipsle main.go
env CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat go build -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o koiAliddns_mips main.go
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOMIPS=softfloat go build -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" -o koiAliddns_x86 main.go
