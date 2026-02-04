APP_NAME := arkana
OUT_DIR  := out

.PHONY: build run test clean tidy

build:
	go build -o $(OUT_DIR)/$(APP_NAME) .

run: build
	./$(OUT_DIR)/$(APP_NAME)

test:
	go test ./...

clean:
	rm -rf $(OUT_DIR)

tidy:
	go mod tidy
