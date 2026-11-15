package protodescriptorcheck

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/descriptorpb"
)

type descriptorFixture struct {
	source         string
	generated      string
	descriptorName string
}

var fixtures = []descriptorFixture{
	{"proto/node.proto", "models/pb/node.pb.go", "node.proto"},
	{"proto/wsc.proto", "models/pb/wsc.pb.go", "wsc.proto"},
}

var (
	goPackage    = regexp.MustCompile("(?m)^\\s*option\\s+go_package\\s*=\\s*\"([^\"]+)\"\\s*;")
	protoPackage = regexp.MustCompile("(?m)^\\s*package\\s+([a-zA-Z_][a-zA-Z0-9_.]*)\\s*;")
)

func TestGeneratedDescriptors(t *testing.T) {
	seen := make(map[string]bool)
	for _, fixture := range fixtures {
		t.Run(fixture.descriptorName, func(t *testing.T) {
			source, err := os.ReadFile(repositoryFile(fixture.source))
			if err != nil {
				t.Fatal(err)
			}
			wantGoPackage := goPackage.FindSubmatch(source)
			if len(wantGoPackage) != 2 {
				t.Fatal("expected go_package in the checked-in proto")
			}
			fields, raw := sourceDescriptor(t, repositoryFile(fixture.generated))
			var fd descriptorpb.FileDescriptorProto
			if err := proto.Unmarshal(raw, &fd); err != nil {
				t.Fatal("invalid raw descriptor", err)
			}
			if fd.GetName() != fixture.descriptorName || seen[fd.GetName()] {
				t.Fatalf("unexpected or duplicate descriptor path %q", fd.GetName())
			}
			seen[fd.GetName()] = true
			if fd.GetOptions().GetGoPackage() != string(wantGoPackage[1]) {
				t.Fatalf("go_package differs from %s", fixture.source)
			}
			wantProtoPackage := ""
			if matches := protoPackage.FindSubmatch(source); len(matches) == 2 {
				wantProtoPackage = string(matches[1])
			}
			if fd.GetPackage() != wantProtoPackage {
				t.Fatalf("protobuf package differs from %s", fixture.source)
			}
			if builderPanic("example.invalid/descriptorcheck", fields, raw) {
				t.Fatal("native protobuf descriptor builder panicked")
			}
		})
	}
}

func repositoryFile(name string) string {
	return filepath.Join("..", "..", filepath.FromSlash(name))
}

func sourceDescriptor(t *testing.T, file string) (*ast.File, []byte) {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			v, ok := spec.(*ast.ValueSpec)
			if !ok || len(v.Values) != len(v.Names) {
				continue
			}
			for i, symbol := range v.Names {
				if !strings.HasSuffix(symbol.Name, "_rawDesc") {
					continue
				}
				var buf bytes.Buffer
				if raw != nil || !descriptorLiteral(v.Values[i], &buf) {
					t.Fatal("expected one static raw descriptor")
				}
				raw = buf.Bytes()
			}
		}
	}
	if len(raw) == 0 {
		t.Fatal("raw descriptor not found")
	}
	return parsed, raw
}

func descriptorLiteral(node ast.Expr, dst *bytes.Buffer) bool {
	switch text := node.(type) {
	case *ast.BasicLit:
		if text.Kind != token.STRING {
			return false
		}
		value, err := strconv.Unquote(text.Value)
		if err != nil {
			return false
		}
		dst.WriteString(value)
		return true
	case *ast.BinaryExpr:
		return text.Op == token.ADD && descriptorLiteral(text.X, dst) && descriptorLiteral(text.Y, dst)
	case *ast.CompositeLit:
		for _, el := range text.Elts {
			lit, ok := el.(*ast.BasicLit)
			if !ok || lit.Kind != token.INT {
				return false
			}
			value, err := strconv.ParseUint(lit.Value, 0, 8)
			if err != nil {
				return false
			}
			dst.WriteByte(byte(value))
		}
		return true
	}
	return false
}

func builderPanic(modulePath string, tree *ast.File, raw []byte) (panicked bool) {
	defer func() { panicked = recover() != nil }()
	size := map[string]int{}
	ast.Inspect(tree, func(node ast.Node) bool {
		p, ok := node.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		k, ok := p.Key.(*ast.Ident)
		if !ok {
			return true
		}
		switch k.Name {
		case "NumEnums", "NumMessages", "NumExtensions", "NumServices":
			if value, ok := p.Value.(*ast.BasicLit); ok && value.Kind == token.INT {
				size[k.Name], _ = strconv.Atoi(value.Value)
			}
		}
		return true
	})
	d := protoimpl.DescBuilder{
		GoPackagePath: modulePath, RawDescriptor: raw, FileRegistry: new(protoregistry.Files),
		NumEnums: int32(size["NumEnums"]), NumMessages: int32(size["NumMessages"]), NumExtensions: int32(size["NumExtensions"]), NumServices: int32(size["NumServices"]),
	}
	_ = d.Build()
	return false
}
