BINARY_NAME=goclaw.exe
BUILD_DIR=bin

all: build

build: clean build-frontend
	go build -o $(BINARY_NAME) ./cmd/goclaw

build-frontend:
	@echo "Building frontend..."
	# In a real project with npm:
	# cd web && npm install && npm run build
	# For this implementation, we use the pre-built index.html in web/dist/

test:
	go test ./...

clean:
	if [ -f $(BINARY_NAME) ]; then rm $(BINARY_NAME); fi
	if [ -d $(BUILD_DIR) ]; then rm -rf $(BUILD_DIR); fi

run: build
	./$(BINARY_NAME)

dev:
	go run ./cmd/goclaw

.PHONY: all build build-frontend test clean run dev
