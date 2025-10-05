APP := confb

.PHONY: build clean

build:
	# build a small binary into ./bin/
	go build -trimpath \
		-ldflags "-s -w -X main.version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
		-o bin/$(APP) ./cmd/confb

clean:
	rm -rf bin
