APP_NAME := arkana
OUT_DIR  := out
DB_PATH  ?= blog.db

.PHONY: build run test clean tidy seed

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

seed:
	@echo "Seeding database at $(DB_PATH)..."
	sqlite3 $(DB_PATH) < scripts/seed.sql
	@echo "Done."
