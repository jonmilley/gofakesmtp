BINARY := gofakesmtp
LDFLAGS := -ldflags="-s -w"

.PHONY: build build-small clean test test-integration

build:
	go build $(LDFLAGS) -o $(BINARY) .

# Further compress the binary with UPX (~60% smaller).
# Requires: brew install upx
build-small: build
	upx --best --lzma $(BINARY)

clean:
	rm -f $(BINARY)

test:
	go test ./...

test-integration:
	go test -tags integration -v ./...
