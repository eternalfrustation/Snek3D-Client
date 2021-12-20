release: deps
	go fmt
	go mod tidy
	go build -o build/ -ldflags "-s -w"

debug: deps
	go fmt
	go mod tidy
	go build -o build/ -ldflags "-n"

deps:
	mkdir -p build/
	cp ico.png build/
	cp frag.frag build/
	cp vertex.vert build/

clean:
	rm -rf build/
