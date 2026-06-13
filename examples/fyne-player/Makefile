.PHONY: build run

build:
	CGO_CFLAGS="-D_GNU_SOURCE" go build -tags wayland -o fyne-player .

run:
	CGO_CFLAGS="-D_GNU_SOURCE" go run -tags wayland main.go
