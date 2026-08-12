package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestTrivialWrapperBranches(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "return forward",
			src: `package p
import "fmt"
func Print(a ...any) (int, error) { return fmt.Print(a...) }
`,
			want: true,
		},
		{
			name: "expr forward",
			src: `package p
import "fmt"
func Do(x string) { fmt.Println(x) }
`,
			want: true,
		},
		{
			name: "multi statement",
			src: `package p
import "fmt"
func Do(x string) { y := x; fmt.Println(y) }
`,
			want: false,
		},
		{
			name: "return zero values",
			src: `package p
func Do() { return }
`,
			want: false,
		},
		{
			name: "return multi expr",
			src: `package p
func Do() (int, int) { return 1, 2 }
`,
			want: false,
		},
		{
			name: "return non call",
			src: `package p
func Do() int { return 1 }
`,
			want: false,
		},
		{
			name: "local call",
			src: `package p
func helper(x string) {}
func Do(x string) { helper(x) }
`,
			want: false,
		},
		{
			name: "args mismatch",
			src: `package p
import "fmt"
func Do(x string) { fmt.Println("x") }
`,
			want: false,
		},
		{
			name: "anonymous param",
			src: `package p
import "fmt"
func Do(string) { fmt.Println("x") }
`,
			want: false,
		},
		{
			name: "renamed import",
			src: `package p
import f "fmt"
func Do(x string) { f.Println(x) }
`,
			want: true,
		},
		{
			name: "dot and blank imports ignored",
			src: `package p
import (
  . "strings"
  _ "os"
  "fmt"
)
func Do(x string) { fmt.Println(x) }
`,
			want: true,
		},
		{
			name: "unexported skipped by checker but helper false",
			src: `package p
import "fmt"
func do(x string) { fmt.Println(x) }
`,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, "p.go", tc.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			imported := importedPackageNames(parsed)
			var fn *ast.FuncDecl
			for _, decl := range parsed.Decls {
				if candidate, ok := decl.(*ast.FuncDecl); ok && candidate.Recv == nil && candidate.Name != nil && isExportedIdent(candidate.Name.Name) {
					fn = candidate
					break
				}
			}
			if fn == nil {
				if tc.want {
					t.Fatal("missing exported func")
				}
				return
			}
			if got := isTrivialWrapper(fn, imported); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
