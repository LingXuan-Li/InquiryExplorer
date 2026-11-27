package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

func main() {
	modPath, root := os.Args[1], os.Args[2]
	tmp, _ := os.MkdirTemp("", "ov")
	replace := map[string]string{}

	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", ".git", "timing", "instrument":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, nil, parser.ParseComments)
		if err != nil {
			return nil
		}
		changed := false
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			name := f.Name.Name + "." + fd.Name.Name
			if fd.Recv != nil && len(fd.Recv.List) > 0 {
				name = f.Name.Name + "." + types.ExprString(fd.Recv.List[0].Type) + "." + fd.Name.Name
			}
			call := &ast.CallExpr{
				Fun: &ast.CallExpr{
					Fun:  &ast.SelectorExpr{X: ast.NewIdent("__timing"), Sel: ast.NewIdent("Track")},
					Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(name)}},
				},
			}
			stmts := []ast.Stmt{&ast.DeferStmt{Call: call}}
			if f.Name.Name == "main" && fd.Name.Name == "main" && fd.Recv == nil {
				flush := &ast.DeferStmt{Call: &ast.CallExpr{
					Fun: &ast.SelectorExpr{X: ast.NewIdent("__timing"), Sel: ast.NewIdent("Flush")},
				}}
				// deferはLIFOなので、先頭に置くと最後に実行される
				stmts = append([]ast.Stmt{flush}, stmts...)
			}
			fd.Body.List = append(stmts, fd.Body.List...)
			changed = true
		}
		if !changed {
			return nil
		}
		astutil.AddNamedImport(fset, f, "__timing", modPath+"/timing")
		abs, _ := filepath.Abs(p)
		out := filepath.Join(tmp, strconv.Itoa(len(replace))+".go")
		w, _ := os.Create(out)
		printer.Fprint(w, fset, f)
		w.Close()
		replace[abs] = out
		return nil
	})

	b, _ := json.Marshal(map[string]any{"Replace": replace})
	os.WriteFile("overlay.json", b, 0o644)
}
