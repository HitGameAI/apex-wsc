package wscpb

// 使用 protoc 24.3、protoc-gen-go v1.36.11 和 protoc-gen-go-grpc v1.6.1。
//go:generate protoc --proto_path=../../proto --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative node.proto wsc.proto
