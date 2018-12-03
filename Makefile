.PHONY: all build test tf clean image dist

BUILD_VERSION ?= manual
BUILD_FLAGS := -a -ldflags '-extldflags "-static"  -X main.version=${BUILD_VERSION}'
BUILDOUTS ?= akms akmsproxy

all: dist

dist: clean test build image

clean:
	for x in ${BUILDOUTS}; do \
	  rm -f ./$$x ; \
	done

build:
	for x in ${BUILDOUTS}; do \
	  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) ./cmd/$$x ; \
	done

test:
#	go test -race ./...

# Test fast
tf:
	go test -short ./...

image:
	for x in ${BUILDOUTS}; do \
	    docker build --pull -t "alpetest/$$x:${BUILD_VERSION}" -f ./cmd/$$x/Dockerfile . ; \
	done


