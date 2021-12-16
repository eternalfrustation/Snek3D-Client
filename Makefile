release:
	go fmt
	go mod tidy
	go build -ldflags "-s -w"

debug:
	go fmt
	go mod tidy
	go build -ldflags "-n"
