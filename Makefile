# Define variables
BUILD_DIR = build
DATA_DIR = $(BUILD_DIR)/data

# Target to compile and build the project
build:
	mkdir -p $(DATA_DIR)
	go build -o $(BUILD_DIR)/

# Target to run the built executable
run: build
	./$(BUILD_DIR)/repository_manager

# Target to clean up the build directory
clean:
	rm -rf $(BUILD_DIR)

.PHONY: build run clean