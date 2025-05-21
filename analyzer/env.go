package analyzer

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

type EnvReport struct {
	envVars      map[string]bool
	dynamicCount uint
}

var envReport *EnvReport

func GetEnvReportInstance() *EnvReport {
	if envReport == nil {
		envReport = &EnvReport{
			envVars:      make(map[string]bool, 0),
			dynamicCount: 0,
		}
	}
	return envReport
}

// Analyze the ast of the source files of packages in pkgs,
// reporting any calls that read the environment variables.
func reportCallsReadingEnv(pkgs []*packages.Package) {
	forEachPackageIncludingDependencies(pkgs, func(p *packages.Package) {
		for _, file := range p.Syntax {
			for _, node := range file.Decls {
				pre := func(c *astutil.Cursor) bool {
					obj, ok := isReadingEnv(p.TypesInfo, c.Node())
					if !ok {
						// This was not a call to a relevant function or method.
						return true
					}

					if obj == nil {
						// Call to Environ, no arguments
						GetEnvReportInstance().dynamicCount += 1
						return true
					}

					switch v := obj.(type) {
					case *ast.BasicLit:
						GetEnvReportInstance().envVars[v.Value] = true
					case *ast.Ident:
						if id, ok := p.TypesInfo.Uses[v]; ok {
							switch idObj := id.(type) {
							case *types.Const:
								val := idObj.Val().String()
								GetEnvReportInstance().envVars[val] = true
							default:
								GetEnvReportInstance().dynamicCount += 1
							}
						}
					default:
						GetEnvReportInstance().dynamicCount += 1
					}

					return true
				}
				astutil.Apply(node, pre, nil)
			}
		}
	})
}

// isReadingEnv checks if node is a statement calling os.Getenv, os.Environ,
// or os.LookupEnv or syscall.Getenv. If so, it returns the argument to that function.
// Otherwise, it returns nil.
func isReadingEnv(typeInfo *types.Info, node ast.Node) (ast.Expr, bool) {
	expr, ok := node.(*ast.ExprStmt)
	if !ok {
		// Not a statement node.
		return nil, false
	}
	call, ok := expr.X.(*ast.CallExpr)
	if !ok {
		// Not a function call.
		return nil, false
	}
	callee, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		// The function to be called is not a selection, so it can't be a call to
		// the sort package.  (Unless the user has dot-imported "sort", but we
		// don't need to worry much about false negatives in unusual cases here.)
		return nil, false
	}
	pkgIdent, ok := callee.X.(*ast.Ident)
	if !ok {
		// The left-hand-side of the selection is not a plain identifier.
		return nil, false
	}
	pkgName, ok := typeInfo.Uses[pkgIdent].(*types.PkgName)
	if !ok {
		// The identifier does not refer to a package.
		return nil, false
	}
	pkgNamePath := pkgName.Imported().Path()
	if pkgNamePath != "os" && pkgNamePath != "syscall" {
		return nil, false
	}
	if name := callee.Sel.Name; name != "Getenv" && name != "Environ" && name != "LookupEnv" {
		return nil, false
	}

	if callee.Sel.Name == "Environ" {
		return nil, true
	}

	if len(call.Args) != 1 {
		return nil, false
	}

	return call.Args[0], true
}
