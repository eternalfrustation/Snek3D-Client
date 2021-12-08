release:
	go fmt
	go mod tidy
	go build -ldflags "-s -w"
build:
	go fmt
	go mod tidy
	go build
