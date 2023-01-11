.PHONY: generate install bench bench-arm64 run run-arm64

all: generate install

generate:
	go generate github.com/stapelberg/scan2drive \
		github.com/stapelberg/scan2drive/proto \
		github.com/stapelberg/scan2drive/templates

install:
	GOARCH=arm64 go install github.com/stapelberg/scan2drive/cmd/scan2drive

test:
	go test -mod=mod -v github.com/stapelberg/scan2drive/...

# notably does not include the neonjpeg encode bench
bench:
	go test -v -bench=. -count=1 github.com/stapelberg/scan2drive/...

bench-arm64:
	GOARCH=arm64 go test -c github.com/stapelberg/scan2drive/internal/neonjpeg
	cpu -host=scan2drive home/michael/go/src/github.com/stapelberg/scan2drive/neonjpeg.test -test.bench=.

run: test
	go run -mod=mod github.com/stapelberg/scan2drive/cmd/scan2drive

run-arm64: test
	cd cmd/scan2drive && CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 gok -i scan2drive run
