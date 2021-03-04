VERSION=0.0.8
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION}"
GO111MODULE=on

all: http-dump-request

.PHONY: http-dump-request

http-dump-request: main.go public/*
	statik -src=public
	go build $(LDFLAGS) -o http-dump-request

clean:
	rm -rf http-dump-request

check:
	go test ./...

tag:
	git tag v${VERSION}
	git push origin v${VERSION}
	git push origin HEAD
