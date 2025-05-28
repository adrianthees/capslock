package analyzer

import (
	"go/ast"
	"go/types"

	"github.com/google/capslock/proto"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

type EnvReport map[*packages.Package]map[string]struct{}

var envReport *EnvReport

func (report EnvReport) Add(pkg *packages.Package, key string) {
	_, ok := report[pkg]
	if !ok {
		report[pkg] = make(map[string]struct{})
	}
	report[pkg][key] = struct{}{}
}

func GetEnvReportInstance() *EnvReport {
	if envReport == nil {
		report := EnvReport(make(map[*packages.Package]map[string]struct{}))
		envReport = &report
	}
	return envReport
}

// Remove specific prefix from string
// if the string does not start with the prefix, it is returned unchanged.
func removePrefix(s string, l string) string {
	if len(s) < len(l) || s[:len(l)] != l {
		return s
	}
	return s[len(l):]
}

// Remove specific postfix from string
// if the string does not end with the prefix, it is returned unchanged.
func removePostfix(s string, l string) string {
	if len(s) < len(l) || s[len(s)-len(l):] != l {
		return s
	}
	return s[:len(s)-len(l)]
}

// Remove quotes from the beginning and end of a string
func trimQuotes(s string) string {
	return removePrefix(removePostfix(s, "\""), "\"")
}

func (report EnvReport) EnvVarInfo() []*proto.EnvVarInfo {
	envVars := make([]*proto.EnvVarInfo, 0)
	for pkg, vars := range report {
		for key := range vars {
			envVars = append(envVars, &proto.EnvVarInfo{
				PackagePath: &pkg.PkgPath,
				VarName:     &key,
			})
		}
	}
	return envVars
}

// Simplified version of EnvReport that returns
// a map of environment variable names to their counts
func (report EnvReport) EnvVarCounts() map[string]int64 {
	if len(report) == 0 {
		return nil
	}
	counts := make(map[string]int64)
	for _, vars := range report {
		for key := range vars {
			counts[key]++
		}
	}
	return counts
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
						GetEnvReportInstance().Add(p, "=DYNAMIC=")

						return true
					}

					switch v := obj.(type) {
					case *ast.BasicLit:
						val := trimQuotes(v.Value)
						GetEnvReportInstance().Add(p, val)
					case *ast.Ident:
						if id, ok := p.TypesInfo.Uses[v]; ok {
							switch idObj := id.(type) {
							case *types.Const:
								val := trimQuotes(idObj.Val().String())
								GetEnvReportInstance().Add(p, val)
							default:
								GetEnvReportInstance().Add(p, "=DYNAMIC=")
							}
						}
					default:
						GetEnvReportInstance().Add(p, "=DYNAMIC=")
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
