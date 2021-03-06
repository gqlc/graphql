package parser

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/gqlc/graphql/ast"
	"github.com/gqlc/graphql/lexer"
	"github.com/gqlc/graphql/token"
)

var (
	gqlFile   string
	exDocFile string

	gqlSrc []byte
	exDoc  ast.Document

	update bool
)

func init() {
	flag.StringVar(&gqlFile, "gqlFile", "testdir/test.gql", "Specify a .gql file for testing/benchmarking")
	flag.StringVar(&exDocFile, "exDocFile", "doc.json", "Specify a .json file that contains the parse tree (ast.Document) for the .gql file")

	flag.BoolVar(&update, "update", false, "Update exDocFile")
}

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	flag.Parse()

	if !filepath.IsAbs(gqlFile) {
		gqlFile = filepath.Join(wd, gqlFile)
	}

	gqlSrc, err = ioutil.ReadFile(gqlFile)
	if err != nil {
		panic(err)
	}

	if !filepath.IsAbs(exDocFile) {
		exDocFile = filepath.Join(wd, exDocFile)
	}

	b, err := ioutil.ReadFile(exDocFile)
	if err != nil {
		panic(err)
	}

	err = jsonpb.Unmarshal(bytes.NewReader(b), &exDoc)
	if err != nil && !update {
		panic(err)
	}

	os.Exit(m.Run())
}

func TestUpdate(t *testing.T) {
	if !update {
		t.Skipf("not updating parse tree file: %s", exDocFile)
		return
	}
	t.Logf("updating parse tree file: %s", exDocFile)

	doc, err := ParseDoc(token.NewDocSet(), "test", bytes.NewReader(gqlSrc), ParseComments)
	if err != nil {
		t.Error(err)
		return
	}

	f, err := os.OpenFile(exDocFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		t.Error(err)
		return
	}

	m := &jsonpb.Marshaler{Indent: "  ", EnumsAsInts: true}
	err = m.Marshal(f, doc)
	if err != nil {
		t.Error(err)
		return
	}
}

func TestParseValue(t *testing.T) {
	t.Run("Basic", func(subT *testing.T) {
		topLvlDirectives := `@test(one: 1, two: "2", thr: 3.0, four: true)`
		doc, err := parse("ParseValue:Basic", topLvlDirectives)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Directives) != 1 {
			subT.Fail()
			return
		}

		d := doc.Directives[0]
		if len(d.Args.Args) != 4 {
			subT.Fail()
			return
		}

		vals := map[string]ast.BasicLit{
			"one":  {Kind: token.INT, Value: "1"},
			"two":  {Kind: token.STRING, Value: `"2"`},
			"thr":  {Kind: token.FLOAT, Value: "3.0"},
			"four": {Kind: token.IDENT, Value: "true"},
		}
		for _, arg := range d.Args.Args {
			b, ok := arg.Value.(*ast.Arg_BasicLit)
			if !ok {
				subT.Fail()
				return
			}

			v, exists := vals[arg.Name.Name]
			if !exists {
				subT.Fail()
				return
			}

			if b.BasicLit.Kind != v.Kind && b.BasicLit.Value != v.Value {
				subT.Fail()
				return
			}
		}
	})

	t.Run("List", func(subT *testing.T) {
		topLvlDirectives := `@test(one: [1, "1", 1.1, true])`
		doc, err := parse("ParseValue:List", topLvlDirectives)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Directives) != 1 {
			subT.Fail()
			return
		}

		d := doc.Directives[0]
		if len(d.Args.Args) != 1 {
			subT.Fail()
			return
		}

		arg := d.Args.Args[0]
		c, ok := arg.Value.(*ast.Arg_CompositeLit)
		if !ok {
			subT.Fail()
			return
		}

		l, ok := c.CompositeLit.Value.(*ast.CompositeLit_ListLit)
		if !ok {
			subT.Fail()
			return
		}

		cl, ok := l.ListLit.List.(*ast.ListLit_CompositeList)
		if !ok {
			subT.Fail()
			return
		}

		vals := map[string]int{"1": 0, `"1"`: 0, "1.1": 0, "true": 0}
		for _, e := range cl.CompositeList.Values {
			switch v := e.Value.(type) {
			case *ast.CompositeLit_BasicLit:
				delete(vals, v.BasicLit.Value)
			}
		}
		if len(vals) > 0 {
			subT.Fail()
			return
		}
	})

	t.Run("Obj", func(subT *testing.T) {
		topLvlDirectives := `@test(one: {one: 1, two: "2", thr: 3.0, four: true, five: [], six: {}})`
		doc, err := parse("ParseValue:Obj", topLvlDirectives)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Directives) != 1 {
			subT.Fail()
			return
		}

		d := doc.Directives[0]
		if len(d.Args.Args) != 1 {
			subT.Fail()
			return
		}

		arg := d.Args.Args[0]
		c, ok := arg.Value.(*ast.Arg_CompositeLit)
		if !ok {
			subT.Fail()
			return
		}

		l, ok := c.CompositeLit.Value.(*ast.CompositeLit_ObjLit)
		if !ok {
			subT.Fail()
			return
		}

		vals := map[string]int{"one": 0, "two": 0, "thr": 0, "four": 0, "five": 0, "six": 0}
		for _, p := range l.ObjLit.Fields {
			delete(vals, p.Key.Name)
		}
		if len(vals) > 0 {
			subT.Fail()
			return
		}
	})
}

func TestDirectives(t *testing.T) {
	testCases := []struct {
		Name string
		Src  string
		Ex   []*ast.DirectiveLit
	}{
		{
			Name: "SimpleList",
			Src:  `@a @b @c`,
			Ex: []*ast.DirectiveLit{
				{
					AtPos: 1,
					Name:  "a",
				},
				{
					AtPos: 4,
					Name:  "b",
				},
				{
					AtPos: 7,
					Name:  "c",
				},
			},
		},
		{
			Name: "WithArgs",
			Src:  `@a() @b(a: 1) @c(a: 1, b: "2", c: 2.4, d: [1,2,3], e: {hello: "world!"})`,
			Ex: []*ast.DirectiveLit{
				{
					AtPos: 1,
					Name:  "a",
					Args: &ast.CallExpr{
						Lparen: 3,
						Rparen: 4,
					},
				},
				{
					AtPos: 6,
					Name:  "b",
					Args: &ast.CallExpr{
						Lparen: 8,
						Args: []*ast.Arg{
							{
								Name:  &ast.Ident{NamePos: 9, Name: "a"},
								Value: &ast.Arg_BasicLit{BasicLit: &ast.BasicLit{Kind: token.INT, ValuePos: 12, Value: "1"}},
							},
						},
						Rparen: 13,
					},
				},
				{
					AtPos: 15,
					Name:  "c",
					Args: &ast.CallExpr{
						Lparen: 17,
						Args: []*ast.Arg{
							{
								Name: &ast.Ident{NamePos: 18, Name: "a"},
								Value: &ast.Arg_BasicLit{BasicLit: &ast.BasicLit{
									Kind:     token.INT,
									ValuePos: 21,
									Value:    "1",
								}},
							},
							{
								Name: &ast.Ident{NamePos: 24, Name: "b"},
								Value: &ast.Arg_BasicLit{BasicLit: &ast.BasicLit{
									Kind:     token.STRING,
									ValuePos: 27,
									Value:    "\"2\"",
								}},
							},
							{
								Name: &ast.Ident{NamePos: 32, Name: "c"},
								Value: &ast.Arg_BasicLit{BasicLit: &ast.BasicLit{
									Kind:     token.FLOAT,
									ValuePos: 35,
									Value:    "2.4",
								}},
							},
							{
								Name: &ast.Ident{NamePos: 40, Name: "d"},
								Value: &ast.Arg_CompositeLit{CompositeLit: &ast.CompositeLit{
									Opening: 43,
									Value: &ast.CompositeLit_ListLit{ListLit: &ast.ListLit{
										List: &ast.ListLit_CompositeList{CompositeList: &ast.ListLit_Composite{
											Values: []*ast.CompositeLit{
												{
													Value: &ast.CompositeLit_BasicLit{
														BasicLit: &ast.BasicLit{Kind: token.INT, ValuePos: 44, Value: "1"},
													},
												},
												{
													Value: &ast.CompositeLit_BasicLit{
														BasicLit: &ast.BasicLit{Kind: token.INT, ValuePos: 46, Value: "2"},
													},
												},
												{
													Value: &ast.CompositeLit_BasicLit{
														BasicLit: &ast.BasicLit{Kind: token.INT, ValuePos: 48, Value: "3"},
													},
												},
											},
										}},
									}},
									Closing: 49,
								}},
							},
							{
								Name: &ast.Ident{NamePos: 52, Name: "e"},
								Value: &ast.Arg_CompositeLit{CompositeLit: &ast.CompositeLit{
									Opening: 55,
									Value: &ast.CompositeLit_ObjLit{ObjLit: &ast.ObjLit{
										Fields: []*ast.ObjLit_Pair{
											{
												Key: &ast.Ident{NamePos: 56, Name: "hello"},
												Val: &ast.CompositeLit{
													Value: &ast.CompositeLit_BasicLit{
														BasicLit: &ast.BasicLit{
															Kind:     token.STRING,
															ValuePos: 63,
															Value:    `"world!"`,
														},
													},
												},
											},
										},
									}},
									Closing: 71,
								}},
							},
						},
						Rparen: 72,
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			defer func() {
				r := recover()
				if r != nil {
					rerr, ok := r.(error)
					if !ok {
						subT.Error(r)
						return
					}
					subT.Error(rerr)
				}
			}()

			dset := token.NewDocSet()

			p := new(parser)
			p.pk.Line = -1
			p.l = lexer.Lex(dset.AddDoc(testCase.Name, dset.Base(), len(testCase.Src)), testCase.Src)

			var directives []*ast.DirectiveLit
			p.parseDirectives(&directives)

			if len(directives) != len(testCase.Ex) {
				subT.Fail()
				return
			}

			for i := range directives {
				if !proto.Equal(directives[i], testCase.Ex[i]) {
					subT.Log(i, directives[i])
					subT.Fail()
					return
				}
			}
		})
	}
}

func TestParser(t *testing.T) {
	testCases := []struct {
		Name string
		Src  string
		Ex   *ast.Document
	}{
		{
			Name: "Scalar",
			Src:  `scalar Test`,
			Ex: &ast.Document{
				Types: []*ast.TypeDecl{
					{
						TokPos: 1,
						Tok:    token.SCALAR,
						Spec: &ast.TypeDecl_TypeSpec{TypeSpec: &ast.TypeSpec{
							Name: &ast.Ident{NamePos: 8, Name: "Test"},
							Type: &ast.TypeSpec_Scalar{Scalar: &ast.ScalarType{
								Scalar: 1,
								Name:   &ast.Ident{NamePos: 8, Name: "Test"},
							}},
						}},
					},
				},
			},
		},
		{
			Name: "ScalarWithDirectives",
			Src:  `scalar Test @a @b() @c(a: 1, b: "2", c: 2.4, d: [1,2,3], e: {hello: "world!"})`,
			Ex: &ast.Document{
				Types: []*ast.TypeDecl{
					{
						TokPos: 1,
						Tok:    token.SCALAR,
						Spec: &ast.TypeDecl_TypeSpec{TypeSpec: &ast.TypeSpec{
							Name: &ast.Ident{NamePos: 8, Name: "Test"},
							Type: &ast.TypeSpec_Scalar{Scalar: &ast.ScalarType{
								Scalar: 1,
								Name:   &ast.Ident{NamePos: 8, Name: "Test"},
							}},
							Directives: []*ast.DirectiveLit{
								{
									AtPos: 13,
									Name:  "a",
								},
								{
									AtPos: 16,
									Name:  "b",
									Args: &ast.CallExpr{
										Lparen: 18,
										Rparen: 19,
									},
								},
								{
									AtPos: 21,
									Name:  "c",
									Args: &ast.CallExpr{
										Lparen: 23,
										Args: []*ast.Arg{
											{
												Name: &ast.Ident{NamePos: 24, Name: "a"},
												Value: &ast.Arg_BasicLit{BasicLit: &ast.BasicLit{
													Kind:     token.INT,
													ValuePos: 27,
													Value:    "1",
												}},
											},
											{
												Name: &ast.Ident{NamePos: 30, Name: "b"},
												Value: &ast.Arg_BasicLit{BasicLit: &ast.BasicLit{
													Kind:     token.STRING,
													ValuePos: 33,
													Value:    "\"2\"",
												}},
											},
											{
												Name: &ast.Ident{NamePos: 38, Name: "c"},
												Value: &ast.Arg_BasicLit{BasicLit: &ast.BasicLit{
													Kind:     token.FLOAT,
													ValuePos: 41,
													Value:    "2.4",
												}},
											},
											{
												Name: &ast.Ident{NamePos: 46, Name: "d"},
												Value: &ast.Arg_CompositeLit{CompositeLit: &ast.CompositeLit{
													Opening: 49,
													Value: &ast.CompositeLit_ListLit{ListLit: &ast.ListLit{
														List: &ast.ListLit_CompositeList{CompositeList: &ast.ListLit_Composite{
															Values: []*ast.CompositeLit{
																{
																	Value: &ast.CompositeLit_BasicLit{
																		BasicLit: &ast.BasicLit{Kind: token.INT, ValuePos: 50, Value: "1"},
																	},
																},
																{
																	Value: &ast.CompositeLit_BasicLit{
																		BasicLit: &ast.BasicLit{Kind: token.INT, ValuePos: 52, Value: "2"},
																	},
																},
																{
																	Value: &ast.CompositeLit_BasicLit{
																		BasicLit: &ast.BasicLit{Kind: token.INT, ValuePos: 54, Value: "3"},
																	},
																},
															},
														}},
													}},
													Closing: 55,
												}},
											},
											{
												Name: &ast.Ident{NamePos: 58, Name: "e"},
												Value: &ast.Arg_CompositeLit{CompositeLit: &ast.CompositeLit{
													Opening: 61,
													Value: &ast.CompositeLit_ObjLit{ObjLit: &ast.ObjLit{
														Fields: []*ast.ObjLit_Pair{
															{
																Key: &ast.Ident{NamePos: 62, Name: "hello"},
																Val: &ast.CompositeLit{
																	Value: &ast.CompositeLit_BasicLit{
																		BasicLit: &ast.BasicLit{
																			Kind:     token.STRING,
																			ValuePos: 69,
																			Value:    `"world!"`,
																		},
																	},
																},
															},
														},
													}},
													Closing: 77,
												}},
											},
										},
										Rparen: 78,
									},
								},
							},
						}},
					},
				},
			},
		},
		{
			Name: "NoFields",
			Src:  `type Test {}`,
			Ex: &ast.Document{
				Types: []*ast.TypeDecl{
					{
						TokPos: 1,
						Tok:    token.TYPE,
						Spec: &ast.TypeDecl_TypeSpec{TypeSpec: &ast.TypeSpec{
							Name: &ast.Ident{NamePos: 6, Name: "Test"},
							Type: &ast.TypeSpec_Object{Object: &ast.ObjectType{
								Object: 1,
								Fields: &ast.FieldList{
									Opening: 11,
									Closing: 12,
								},
							}},
						}},
					},
				},
			},
		},
		{
			Name: "WithComments",
			Src: `type Test { # Hello
}`,
			Ex: &ast.Document{
				Types: []*ast.TypeDecl{
					{
						TokPos: 1,
						Tok:    token.TYPE,
						Doc: &ast.DocGroup{List: []*ast.DocGroup_Doc{
							{Text: "# Hello\n", Comment: true, Char: 13},
						}},
						Spec: &ast.TypeDecl_TypeSpec{TypeSpec: &ast.TypeSpec{
							Name: &ast.Ident{NamePos: 6, Name: "Test"},
							Type: &ast.TypeSpec_Object{Object: &ast.ObjectType{
								Object: 1,
								Fields: &ast.FieldList{
									Opening: 11,
									Closing: 21,
								},
							}},
						}},
					},
				},
			},
		},
		{
			Name: "WithInterfaces",
			Src:  `type Test implements A & B & C`,
			Ex: &ast.Document{
				Types: []*ast.TypeDecl{
					{
						TokPos: 1,
						Tok:    token.TYPE,
						Spec: &ast.TypeDecl_TypeSpec{TypeSpec: &ast.TypeSpec{
							Name: &ast.Ident{NamePos: 6, Name: "Test"},
							Type: &ast.TypeSpec_Object{Object: &ast.ObjectType{
								Object:  1,
								ImplPos: 11,
								Interfaces: []*ast.Ident{
									{NamePos: 22, Name: "A"},
									{NamePos: 26, Name: "B"},
									{NamePos: 30, Name: "C"},
								},
							}},
						}},
					},
				},
			},
		},
		{
			Name: "WithInterfacesAndFields",
			Src: `type Test implements A & B & C {
	one(): One
	two(one: One): Two! @one @two
	thr(one: One = 1, two: Two): [Thr]!
	for: [For!]!
}`,
			Ex: &ast.Document{
				Types: []*ast.TypeDecl{
					{
						TokPos: 1,
						Tok:    token.TYPE,
						Spec: &ast.TypeDecl_TypeSpec{TypeSpec: &ast.TypeSpec{
							Name: &ast.Ident{NamePos: 6, Name: "Test"},
							Type: &ast.TypeSpec_Object{Object: &ast.ObjectType{
								Object:  1,
								ImplPos: 11,
								Interfaces: []*ast.Ident{
									{NamePos: 22, Name: "A"},
									{NamePos: 26, Name: "B"},
									{NamePos: 30, Name: "C"},
								},
								Fields: &ast.FieldList{
									Opening: 32,
									List: []*ast.Field{
										{
											Name: &ast.Ident{NamePos: 35, Name: "one"},
											Args: &ast.InputValueList{
												Opening: 38,
												Closing: 39,
											},
											Type: &ast.Field_Ident{
												Ident: &ast.Ident{NamePos: 42, Name: "One"},
											},
										},
										{
											Name: &ast.Ident{NamePos: 47, Name: "two"},
											Args: &ast.InputValueList{
												Opening: 50,
												List: []*ast.InputValue{
													{
														Name: &ast.Ident{NamePos: 51, Name: "one"},
														Type: &ast.InputValue_Ident{
															Ident: &ast.Ident{NamePos: 56, Name: "One"},
														},
													},
												},
												Closing: 59,
											},
											Type: &ast.Field_NonNull{
												NonNull: &ast.NonNull{
													Type: &ast.NonNull_Ident{
														Ident: &ast.Ident{NamePos: 62, Name: "Two"},
													},
												},
											},
											Directives: []*ast.DirectiveLit{
												{
													AtPos: 67,
													Name:  "one",
												},
												{
													AtPos: 72,
													Name:  "two",
												},
											},
										},
										{
											Name: &ast.Ident{NamePos: 78, Name: "thr"},
											Args: &ast.InputValueList{
												Opening: 81,
												List: []*ast.InputValue{
													{
														Name: &ast.Ident{NamePos: 82, Name: "one"},
														Type: &ast.InputValue_Ident{
															Ident: &ast.Ident{NamePos: 87, Name: "One"},
														},
														Default: &ast.InputValue_BasicLit{
															BasicLit: &ast.BasicLit{Kind: token.INT, ValuePos: 93, Value: "1"},
														},
													},
													{
														Name: &ast.Ident{NamePos: 96, Name: "two"},
														Type: &ast.InputValue_Ident{
															Ident: &ast.Ident{NamePos: 101, Name: "Two"},
														},
													},
												},
												Closing: 104,
											},
											Type: &ast.Field_NonNull{
												NonNull: &ast.NonNull{
													Type: &ast.NonNull_List{
														List: &ast.List{
															Type: &ast.List_Ident{
																Ident: &ast.Ident{NamePos: 108, Name: "Thr"},
															},
														},
													},
												},
											},
										},
										{
											Name: &ast.Ident{NamePos: 115, Name: "for"},
											Type: &ast.Field_NonNull{
												NonNull: &ast.NonNull{
													Type: &ast.NonNull_List{
														List: &ast.List{
															Type: &ast.List_NonNull{
																NonNull: &ast.NonNull{
																	Type: &ast.NonNull_Ident{
																		Ident: &ast.Ident{NamePos: 121, Name: "For"},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
									Closing: 128,
								},
							}},
						}},
					},
				},
			},
		},
		{
			Name: "Union",
			Src:  `union Test @a = A | B | C`,
			Ex: &ast.Document{
				Types: []*ast.TypeDecl{
					{
						TokPos: 1,
						Tok:    token.UNION,
						Spec: &ast.TypeDecl_TypeSpec{TypeSpec: &ast.TypeSpec{
							Name: &ast.Ident{NamePos: 7, Name: "Test"},
							Directives: []*ast.DirectiveLit{
								{
									AtPos: 12,
									Name:  "a",
								},
							},
							Type: &ast.TypeSpec_Union{Union: &ast.UnionType{
								Union: 1,
								Members: []*ast.Ident{
									{NamePos: 17, Name: "A"},
									{NamePos: 21, Name: "B"},
									{NamePos: 25, Name: "C"},
								},
							}},
						}},
					},
				},
			},
		},
		{
			Name: "Enum",
			Src: `enum Test {
	A
	B
	C
}`,
			Ex: &ast.Document{
				Types: []*ast.TypeDecl{
					{
						TokPos: 1,
						Tok:    token.ENUM,
						Spec: &ast.TypeDecl_TypeSpec{TypeSpec: &ast.TypeSpec{
							Name: &ast.Ident{NamePos: 6, Name: "Test"},
							Type: &ast.TypeSpec_Enum{Enum: &ast.EnumType{
								Enum: 1,
								Values: &ast.FieldList{
									Opening: 11,
									List: []*ast.Field{
										{Name: &ast.Ident{NamePos: 14, Name: "A"}},
										{Name: &ast.Ident{NamePos: 17, Name: "B"}},
										{Name: &ast.Ident{NamePos: 20, Name: "C"}},
									},
									Closing: 22,
								},
							}},
						}},
					},
				},
			},
		},
		{
			Name: "EnumWithComment",
			Src: `enum Test { # Hello
}`,
			Ex: &ast.Document{
				Types: []*ast.TypeDecl{
					{
						TokPos: 1,
						Tok:    token.ENUM,
						Doc: &ast.DocGroup{List: []*ast.DocGroup_Doc{
							{Text: "# Hello\n", Comment: true, Char: 13},
						}},
						Spec: &ast.TypeDecl_TypeSpec{TypeSpec: &ast.TypeSpec{
							Name: &ast.Ident{NamePos: 6, Name: "Test"},
							Type: &ast.TypeSpec_Enum{Enum: &ast.EnumType{
								Enum: 1,
								Values: &ast.FieldList{
									Opening: 11,
									Closing: 21,
								},
							}},
						}},
					},
				},
			},
		},
		{
			Name: "Directive",
			Src:  `directive @test on SCHEMA | FIELD`,
			Ex: &ast.Document{
				Types: []*ast.TypeDecl{
					{
						TokPos: 1,
						Tok:    token.DIRECTIVE,
						Spec: &ast.TypeDecl_TypeSpec{TypeSpec: &ast.TypeSpec{
							Name: &ast.Ident{NamePos: 12, Name: "test"},
							Type: &ast.TypeSpec_Directive{Directive: &ast.DirectiveType{
								Directive: 1,
								OnPos:     17,
								Locs: []*ast.DirectiveLocation{
									{Start: 20, Loc: ast.DirectiveLocation_SCHEMA},
									{Start: 29, Loc: ast.DirectiveLocation_FIELD},
								},
							}},
						}},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			defer func() {
				r := recover()
				if r != nil {
					rerr, ok := r.(error)
					if !ok {
						subT.Error(r)
						return
					}
					subT.Error(rerr)
				}
			}()

			dset := token.NewDocSet()
			p := &parser{mode: ParseComments}

			doc := dset.AddDoc(testCase.Name, dset.Base(), len(testCase.Src))
			p.l = lexer.Lex(doc, testCase.Src)
			p.doc = doc
			p.pk.Line = -1

			d := new(ast.Document)
			p.parseDoc(&d.Types, &d.Directives)

			compare(subT, d, testCase.Ex)
		})
	}
}

func TestParseDoc(t *testing.T) {
	doc, err := ParseDoc(token.NewDocSet(), "test", bytes.NewReader(gqlSrc), ParseComments)
	if err != nil {
		t.Error(err)
		return
	}

	compare(t, doc, &exDoc)
}

func BenchmarkParseDoc(b *testing.B) {
	dset := token.NewDocSet()
	name := "test"

	for i := 0; i < b.N; i++ {
		_, err := ParseDoc(dset, name, bytes.NewReader(gqlSrc), ParseComments)
		if err != nil {
			b.Error(err)
			return
		}
	}
}

func TestParseDir(t *testing.T) {
	docs, err := ParseDir(token.NewDocSet(), "./testdir", nil, ParseComments)
	if err != nil {
		t.Error(err)
		return
	}

	if len(docs) != 1 {
		t.Log("expected 1 doc")
		t.Fail()
		return
	}
}

func parse(name, src string) (*ast.Document, error) {
	return ParseDoc(token.NewDocSet(), name, strings.NewReader(src), 0)
}

func compare(t *testing.T, out, ex *ast.Document) {
	if proto.Equal(out, ex) {
		return
	}
	t.Fail()

	if len(out.Types) != len(ex.Types) {
		t.Fatalf("mismatched type lengths")
		return
	}

	for i, ext := range ex.Types {
		if !proto.Equal(out.Types[i], ext) {
			t.Logf("Found decl inequality:\nOut: %s\nExp: %s\n", out.Types[i], ext)
		}
	}

	if !proto.Equal(out.Schema, ex.Schema) {
		t.Logf("Found schema inequality:\nOut: %s\nExp: %s\n", out.Schema, ex.Schema)
	}
}
