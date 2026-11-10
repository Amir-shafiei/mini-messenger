package service_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCoreDependencies(t *testing.T) {
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		p := filepath.ToSlash(path)
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") || (!strings.Contains(p, "/model/") && !strings.Contains(p, "/service/")) {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			v, _ := strconv.Unquote(imp.Path.Value)
			allowed := v == "context" || v == "errors" || v == "time" || v == "strings" || v == "unicode/utf8"
			if strings.Contains(p, "/service/") && v == "mini-messenger/internal/model" {
				allowed = true
			}
			if !allowed {
				t.Errorf("forbidden core dependency: %s imports %s", p, v)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
