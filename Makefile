# Define variables
SOURCE_DIR = ./cmd/server
BUILD_DIR = build
DATA_DIR = $(BUILD_DIR)/data
EXECUTABLE = repo-man

# Target to compile and build the project
build:
	mkdir -p $(DATA_DIR)
	go build -o $(BUILD_DIR)/$(EXECUTABLE) ./cmd/server

# Target to run the built executable from within the build directory
run: build
	cd $(BUILD_DIR) && ./$(EXECUTABLE)


# Target to clean up the build directory
clean:
	rm -rf $(BUILD_DIR)

.PHONY: build run clean