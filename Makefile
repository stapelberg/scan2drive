.PHONY: generate install

all: generate install

generate:
	go generate github.com/stapelberg/scan2drive \
		github.com/stapelberg/scan2drive/proto \
		github.com/stapelberg/scan2drive/templates

install:
	GOARCH=arm64 go install github.com/stapelberg/scan2drive

test:
	true

run: test
	go run -mod=mod github.com/stapelberg/scan2drive
