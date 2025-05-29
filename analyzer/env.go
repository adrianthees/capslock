package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/google/capslock/proto"
)

type GetEnvCalls map[string]token.Position

var getEnvCalls *GetEnvCalls

func GetEnvCallsInstance() *GetEnvCalls {
	if getEnvCalls == nil {
		calls := GetEnvCalls(make(map[string]token.Position))
		getEnvCalls = &calls
	}
	return getEnvCalls
}

func (calls *GetEnvCalls) Add(depPath string, pos token.Position) {
	if calls == nil {
		calls = GetEnvCallsInstance()
	}
	(*calls)[depPath] = pos
}

type CallInfo struct {
	DepPath string
	Pos     token.Position
}

func (calls *GetEnvCalls) filesToCallInfo() map[string][]CallInfo {
	result := make(map[string][]CallInfo)
	for depPath, pos := range *calls {
		fileName := pos.Filename
		result[fileName] = append(result[fileName], CallInfo{
			DepPath: depPath,
			Pos:     pos,
		})
	}
	return result
}

func nodesToEnvVarInfo(n map[string]ast.Node) []*proto.EnvVarInfo {
	results := make([]*proto.EnvVarInfo, 0, len(n))
	for depPath, node := range n {
		callExpr, ok := node.(*ast.CallExpr)
		if !ok {
			continue
		}
		callee, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		varName := "=DYNAMIC="
		if callee.Sel.Name != "Environ" {
			switch v := callExpr.Args[0].(type) {
			case *ast.BasicLit:
				varName = trimQuotes(v.Value)

			case *ast.Ident:
				if v.Obj != nil && v.Obj.Kind == ast.Con {
					if valueSpec, ok := v.Obj.Decl.(*ast.ValueSpec); ok && len(valueSpec.Values) > 0 {
						if constLit, ok := valueSpec.Values[0].(*ast.BasicLit); ok {
							varName = trimQuotes(constLit.Value)
						}
					}
				}
			}
		}
		results = append(results, &proto.EnvVarInfo{
			DepPath: &depPath,
			VarName: &varName,
		})
	}
	return results
}

func findCallSiteNodes(callsByFiles map[string][]CallInfo) map[string]ast.Node {
	result := make(map[string]ast.Node)

	for filename, calls := range callsByFiles {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		for _, call := range calls {
			var callNode ast.Node
			ast.Inspect(f, func(n ast.Node) bool {
				if n == nil {
					return true
				}

				pos := fset.Position(n.Pos())
				end := fset.Position(n.End())

				if pos.Filename == call.Pos.Filename &&
					(pos.Line < call.Pos.Line || (pos.Line == call.Pos.Line && pos.Column <= call.Pos.Column)) &&
					(end.Line > call.Pos.Line || (end.Line == call.Pos.Line && end.Column >= call.Pos.Column)) {

					if callNode == nil || (n.End()-n.Pos() < callNode.End()-callNode.Pos()) {
						if _, isCall := n.(*ast.CallExpr); isCall {
							callNode = n
						}
					}
				}
				return true
			})

			if callNode != nil {
				result[call.DepPath] = callNode
			}
		}
	}
	return result
}

func (calls *GetEnvCalls) EnvVarInfo() []*proto.EnvVarInfo {
	callsByFiles := GetEnvCallsInstance().filesToCallInfo()
	nodes := findCallSiteNodes(callsByFiles)
	envVars := nodesToEnvVarInfo(nodes)
	return envVars
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
