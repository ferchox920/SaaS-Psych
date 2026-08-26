package db

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRepositoryQueriesRemainTenantAware(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob go files: %v", err)
	}

	allowedWithoutTenantID := map[string]struct{}{
		"TenantRepository.Exists": {},
	}

	fset := token.NewFileSet()

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}

		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil {
				continue
			}

			receiverName, ok := repositoryReceiverName(fn.Recv)
			if !ok {
				continue
			}

			methodName := receiverName + "." + fn.Name.Name
			if _, allowed := allowedWithoutTenantID[methodName]; allowed {
				continue
			}

			if !functionUsesRepositoryDBCall(fn.Body) {
				continue
			}

			if !functionContainsTenantIDLiteral(fn.Body) {
				t.Errorf("%s in %s performs DB access without tenant_id guard", methodName, file)
			}
		}
	}
}

func repositoryReceiverName(recv *ast.FieldList) (string, bool) {
	if recv == nil || len(recv.List) != 1 {
		return "", false
	}

	switch expr := recv.List[0].Type.(type) {
	case *ast.StarExpr:
		ident, ok := expr.X.(*ast.Ident)
		if !ok || !strings.HasSuffix(ident.Name, "Repository") {
			return "", false
		}
		return ident.Name, true
	case *ast.Ident:
		if !strings.HasSuffix(expr.Name, "Repository") {
			return "", false
		}
		return expr.Name, true
	default:
		return "", false
	}
}

func functionUsesRepositoryDBCall(body *ast.BlockStmt) bool {
	usesDB := false

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if sel.Sel == nil {
			return true
		}

		switch sel.Sel.Name {
		case "Query", "QueryRow", "Exec":
			usesDB = true
			return false
		default:
			return true
		}
	})

	return usesDB
}

func functionContainsTenantIDLiteral(body *ast.BlockStmt) bool {
	found := false

	ast.Inspect(body, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}

		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}

		if strings.Contains(strings.ToLower(value), "tenant_id") {
			found = true
			return false
		}

		return true
	})

	return found
}
