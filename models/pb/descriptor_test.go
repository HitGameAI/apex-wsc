package wscpb_test

import (
	"bytes"
	"testing"

	wscpb "github.com/HitGameAI/apex-wsc/models/pb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestFileDescriptors(t *testing.T) {
	for _, file := range []protoreflect.FileDescriptor{wscpb.File_node_proto, wscpb.File_wsc_proto} {
		t.Run(file.Path(), func(t *testing.T) {
			descriptor := protodesc.ToFileDescriptorProto(file)
			if got, want := descriptor.GetOptions().GetGoPackage(), "github.com/HitGameAI/apex-wsc/models/pb;wscpb"; got != want {
				t.Fatalf("go_package = %q, want %q", got, want)
			}
			raw, err := proto.Marshal(descriptor)
			if err != nil {
				t.Fatal(err)
			}
			var decoded descriptorpb.FileDescriptorProto
			if err := proto.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if _, err := protodesc.NewFile(&decoded, protoregistry.GlobalFiles); err != nil {
				t.Fatalf("rebuild descriptor: %v", err)
			}
		})
	}
}

func TestNodeAndHubMessageRoundTrip(t *testing.T) {
	messages := []proto.Message{
		&wscpb.SendToUserRequest{UserId: "u", MessageData: []byte("m")},
		&wscpb.HubMessageProto{Id: "m", TraceId: "trace", MessageType: "notice", Namespace: "tenant", GroupIds: []string{"group"}, RequireAck: true},
	}
	for _, message := range messages {
		t.Run(string(message.ProtoReflect().Descriptor().FullName()), func(t *testing.T) {
			raw, err := proto.Marshal(message)
			if err != nil {
				t.Fatal(err)
			}
			// 节点间 RPC 的字段编号及编码必须与迁移前保持兼容。
			if _, ok := message.(*wscpb.SendToUserRequest); ok && !bytes.Equal(raw, []byte{0x0a, 1, 'u', 0x12, 1, 'm'}) {
				t.Fatalf("SendToUserRequest wire encoding changed: %x", raw)
			}
			decoded := message.ProtoReflect().Type().New().Interface()
			if err := proto.Unmarshal(raw, decoded); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(message, decoded) {
				t.Fatal("protobuf round-trip changed the message")
			}
			jsonData, err := protojson.Marshal(message)
			if err != nil {
				t.Fatal(err)
			}
			proto.Reset(decoded)
			if err := protojson.Unmarshal(jsonData, decoded); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(message, decoded) {
				t.Fatal("protobuf JSON round-trip changed the message")
			}
		})
	}
}
