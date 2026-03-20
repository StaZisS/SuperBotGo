.PHONY: build clean

build:
	GOOS=wasip1 GOARCH=wasm go build -o schedule.wasm ./

clean:
	rm -f schedule.wasm
