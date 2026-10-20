.PHONY: build test clean run

BINARY=Dmanga.exe
SRC=./cmd/dmanga

build:
	go build -ldflags="-s -w" -o $(BINARY) $(SRC)

run:
	go run $(SRC)

test:
	go test -v ./...

clean:
	if exist $(BINARY) del $(BINARY)
