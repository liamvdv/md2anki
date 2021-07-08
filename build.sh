#!/bin/bash

# This is ment for developers, not users.

# build for linux and darwin
GOOS=linux GOARCH=amd64 go build -o md2anki main.go toggle.go

# build for windows
GOOS=windows GOARCH=amd64 go build -o md2anki.exe main.go toggle.go
