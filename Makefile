APP_NAME=SysProfiler
OUTPUT_DIR=dist

.PHONY: all clean windows macos linux

all: windows macos linux

clean:
	rm -rf $(OUTPUT_DIR)

windows:
	mkdir -p $(OUTPUT_DIR)/windows
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
		go build -ldflags="-H=windowsgui -s -w" -o $(OUTPUT_DIR)/windows/$(APP_NAME).exe ./cmd/sysprofiler

linux:
	mkdir -p $(OUTPUT_DIR)/linux
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 \
		go build -ldflags="-s -w" -o $(OUTPUT_DIR)/linux/$(APP_NAME) ./cmd/sysprofiler

macos:
	mkdir -p $(OUTPUT_DIR)/macos/$(APP_NAME).app/Contents/MacOS
	mkdir -p $(OUTPUT_DIR)/macos/$(APP_NAME).app/Contents/Resources
	# Build Intel x86_64
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=10.13 \
		go build -ldflags="-s -w" -o $(OUTPUT_DIR)/macos/bin_amd64 ./cmd/sysprofiler
	# Build Apple Silicon arm64
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=10.13 \
		go build -ldflags="-s -w" -o $(OUTPUT_DIR)/macos/bin_arm64 ./cmd/sysprofiler
	# Combine into Universal 2 binary
	lipo -create -output $(OUTPUT_DIR)/macos/$(APP_NAME).app/Contents/MacOS/$(APP_NAME) \
		$(OUTPUT_DIR)/macos/bin_amd64 $(OUTPUT_DIR)/macos/bin_arm64
	rm $(OUTPUT_DIR)/macos/bin_amd64 $(OUTPUT_DIR)/macos/bin_arm64
	cp assets/info.plist.template $(OUTPUT_DIR)/macos/$(APP_NAME).app/Contents/Info.plist
	codesign --force --deep -s - $(OUTPUT_DIR)/macos/$(APP_NAME).app
