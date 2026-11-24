//go:build tools

// Package tools pins code generation and build-script tools in go.mod without adding them to runtime builds.
package tools

import (
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
	_ "gopkg.in/yaml.v3"
)
