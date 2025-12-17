// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cel_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/ext"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	proto3pb "github.com/google/cel-go/test/proto3pb"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestStaticOptimizerUpdateExpr(t *testing.T) {
	expr := `has(a.b)`
	inlined := `[x, y].filter(i, i.size() > 0)[0].z`

	e := optimizerEnv(t)
	exprAST, iss := e.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}

	inlinedAST, iss := e.Compile(inlined)
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	opt := cel.NewStaticOptimizer(&testOptimizer{t: t, inlineExpr: inlinedAST.NativeRep()})
	optAST, iss := opt.Optimize(e, exprAST)
	if iss.Err() != nil {
		t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
	}
	optString, err := cel.AstToString(optAST)
	if err != nil {
		t.Fatalf("cel.AstToString() failed: %v", err)
	}
	sourceInfo := optAST.NativeRep().SourceInfo()
	sourceInfoPB, err := ast.SourceInfoToProto(sourceInfo)
	if err != nil {
		t.Fatalf("cel.AstToCheckedExpr() failed: %v", err)
	}
	wantTextPB := `
		location:  "<input>"
        line_offsets:  9
        positions:  {
          key:  2
          value:  4
        }
        positions:  {
          key:  3
          value:  5
        }
        positions:  {
          key:  4
          value:  3
        }
        macro_calls:  {
          key:  1
          value:  {
            call_expr:  {
              function:  "has"
              args:  {
                id:  21
                select_expr:  {
                  operand:  {
                    id:  2
                    call_expr:  {
                      function:  "_[_]"
                      args:  {
                        id:  3
                      }
                      args:  {
                        id:  20
                        const_expr:  {
                          int64_value:  0
                        }
                      }
                    }
                  }
                  field:  "z"
                }
              }
            }
          }
        }
        macro_calls:  {
          key:  3
          value:  {
            call_expr:  {
              target:  {
                id:  4
                list_expr:  {
                  elements:  {
                    id:  5
                    ident_expr:  {
                      name:  "x"
                    }
                  }
                  elements:  {
                    id:  6
                    ident_expr:  {
                      name:  "y"
                    }
                  }
                }
              }
              function:  "filter"
              args:  {
                id:  17
                ident_expr:  {
                  name:  "i"
                }
              }
              args:  {
                id:  10
                call_expr:  {
                  function:  "_>_"
                  args:  {
                    id:  11
                    call_expr:  {
                      target:  {
                        id:  12
                        ident_expr:  {
                          name:  "i"
                        }
                      }
                      function:  "size"
                    }
                  }
                  args:  {
                    id:  13
                    const_expr:  {
                      int64_value:  0
                    }
                  }
                }
              }
            }
          }
        }
	`
	var wantSourceInfoPB exprpb.SourceInfo
	if err := prototext.Unmarshal([]byte(wantTextPB), &wantSourceInfoPB); err != nil {
		t.Fatalf("prototext.Unmarshal() failed: %v", err)
	}
	if !proto.Equal(&wantSourceInfoPB, sourceInfoPB) {
		t.Errorf("got source info: %s, wanted %s", prototext.Format(sourceInfoPB), wantTextPB)
	}
	expected := `has([x, y].filter(i, i.size() > 0)[0].z)`
	if expected != optString {
		t.Errorf("inlined got %q, wanted %q", optString, expected)
	}
}

func TestStaticOptimizerNewAST(t *testing.T) {
	tests := []string{
		`[3, 2, 1]`,
		`[1, 2, 3].all(i, i != 0)`,
		`cel.bind(m, {"a": 1, "b": 2}, m.filter(k, m[k] > 1))`,
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc, func(t *testing.T) {
			e := optimizerEnv(t)
			exprAST, iss := e.Compile(tc)
			if iss.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", tc, iss.Err())
			}
			opt := cel.NewStaticOptimizer(&identityOptimizer{t: t})
			optAST, iss := opt.Optimize(e, exprAST)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			optString, err := cel.AstToString(optAST)
			if err != nil {
				t.Fatalf("cel.AstToString() failed: %v", err)
			}
			if tc != optString {
				t.Errorf("identity optimizer got %q, wanted %q", optString, tc)
			}
		})
	}
}

func TestStaticOptimizerNilAST(t *testing.T) {
	env := optimizerEnv(t)
	opt := cel.NewStaticOptimizer(&identityOptimizer{t: t})
	optAST, iss := opt.Optimize(env, nil)
	if iss.Err() == nil || !strings.Contains(iss.Err().Error(), "unexpected unspecified type") {
		t.Errorf("opt.Optimize(env, nil) got (%v, %v), wanted unexpected unspecified type", optAST, iss)
	}
}

type identityOptimizer struct {
	t *testing.T
}

func (opt *identityOptimizer) Optimize(ctx *cel.OptimizerContext, a *ast.AST) *ast.AST {
	opt.t.Helper()
	// The copy method should effectively update all of the old macro refs with new ones that are
	// identical, but renumbered.
	main := ctx.CopyASTAndMetadata(a)
	// The new AST call will create a parsed expression which will be type-checked by the static
	// optimizer. The input and output expressions should be identical, though may vary by number
	// though.
	return ctx.NewAST(main)
}

type testOptimizer struct {
	t          *testing.T
	inlineExpr *ast.AST
}

func (opt *testOptimizer) Optimize(ctx *cel.OptimizerContext, a *ast.AST) *ast.AST {
	opt.t.Helper()
	copy := ctx.CopyASTAndMetadata(opt.inlineExpr)
	origID := a.Expr().ID()
	presenceTest, hasMacro := ctx.NewHasMacro(origID, copy)
	macroKeys := getMacroKeys(ctx.MacroCalls())
	if len(macroKeys) != 2 {
		opt.t.Errorf("Got %v macro calls, wanted 2", macroKeys)
	}
	ctx.UpdateExpr(a.Expr(), presenceTest)
	ctx.SetMacroCall(origID, hasMacro)
	return ctx.NewAST(a.Expr())
}

func getMacroKeys(macroCalls map[int64]ast.Expr) []int {
	keys := []int{}
	for k := range macroCalls {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	return keys
}

func optimizerEnv(t *testing.T) *cel.Env {
	t.Helper()
	opts := []cel.EnvOption{
		cel.Types(&proto3pb.TestAllTypes{}),
		cel.OptionalTypes(),
		cel.EnableMacroCallTracking(),
		ext.Bindings(),
		cel.Variable("a", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("x", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("y", cel.MapType(cel.StringType, cel.StringType)),
	}
	e, err := cel.NewEnv(opts...)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	return e
}
