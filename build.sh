#!/bin/bash

NAME="packly"
VERSION="1.0.0"

echo "Starting multi-platform build for $NAME $VERSION..."

# Linux
echo "Building for Linux..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o build/${NAME}-linux
if [ $? -eq 0 ]; then
    echo "Build successful for Linux: ${NAME}-linux"
else
    echo "Build failed for Linux"
    exit 1
fi

# Windows
echo "Building for Windows..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o build/${NAME}-windows.exe
if [ $? -eq 0 ]; then
    echo "Build successful for Windows: ${NAME}-windows.exe"
else
    echo "Build failed for Windows"
    exit 1
fi

# MacOS
echo "Building for MacOS..."
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o build/${NAME}-macos
if [ $? -eq 0 ]; then
    echo "Build successful for MacOS: ${NAME}-macos"
else
    echo "Build failed for MacOS"
    exit 1
fi

echo "✅ Done! All binaries are in the 'build' folder."