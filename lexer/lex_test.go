package lexer

import (
	"encoding/json"
	"flag"
	"github.com/gqlc/graphql/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

var (
	gqlFile       string
	testItemsFile string

	gqlSrc    []byte
	testItems struct{ Items []Item }
)

func init() {
	flag.StringVar(&gqlFile, "gqlFile", "test.gql", "Specify a .gql file for testing/benchmarking")
	flag.StringVar(&testItemsFile, "itemsFile", "items.json", "Specify a .json file that contains the lexed items for the .gql file")
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

	if !filepath.IsAbs(testItemsFile) {
		testItemsFile = filepath.Join(wd, testItemsFile)
	}

	b, err := ioutil.ReadFile(testItemsFile)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(b, &testItems)
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func expectItems(t *testing.T, l Interface, items ...Item) {
	for _, item := range items {
		lItem := l.NextItem()
		if lItem.Typ != item.Typ || lItem.Val != item.Val {
			t.Fatalf("expected item: %#v but instead received: %#v", item, lItem)
		}
	}
}

func expectEOF(t *testing.T, l Interface) {
	i := l.NextItem()
	if i.Typ != token.EOF {
		t.Fatalf("expected eof but instead received: %#v", i)
	}
}

func TestLexer(t *testing.T) {
	testCases := []struct {
		Name  string
		Src   string
		Items []Item
	}{
		{
			Name: "Schema",
			Src:  `schema {}`,
			Items: []Item{
				{Typ: token.SCHEMA, Val: "schema"},
				{Typ: token.LBRACE, Val: "{"},
				{Typ: token.RBRACE, Val: "}"},
			},
		},
		{
			Name: "SchemaWithDirectives",
			Src:  `schema @a @b() @c(a: 1, b: "2", c: [1, 2, 3]) {}`,
			Items: []Item{
				{Typ: token.SCHEMA, Val: "schema"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.INT, Val: "1"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.STRING, Val: "\"2\""},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.LBRACK, Val: "["},
				{Typ: token.INT, Val: "1"},
				{Typ: token.INT, Val: "2"},
				{Typ: token.INT, Val: "3"},
				{Typ: token.RBRACK, Val: "]"},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.LBRACE, Val: "{"},
				{Typ: token.RBRACE, Val: "}"},
			},
		},
		{
			Name: "TypeDecl",
			Src:  `scalar Test`,
			Items: []Item{
				{Typ: token.SCALAR, Val: "scalar"},
				{Typ: token.IDENT, Val: "Test"},
			},
		},
		{
			Name: "TypeDeclWithDirectives",
			Src:  `scalar Test @a @b() @c(a: 1, b: "2", c: [1, 2, 3])`,
			Items: []Item{
				{Typ: token.SCALAR, Val: "scalar"},
				{Typ: token.IDENT, Val: "Test"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.INT, Val: "1"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.STRING, Val: "\"2\""},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.LBRACK, Val: "["},
				{Typ: token.INT, Val: "1"},
				{Typ: token.INT, Val: "2"},
				{Typ: token.INT, Val: "3"},
				{Typ: token.RBRACK, Val: "]"},
				{Typ: token.RPAREN, Val: ")"},
			},
		},
		{
			Name: "TypeDeclWithInterfaces",
			Src:  `type Test implements A & B & C`,
			Items: []Item{
				{Typ: token.TYPE, Val: "type"},
				{Typ: token.IDENT, Val: "Test"},
				{Typ: token.IMPLEMENTS, Val: "implements"},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.IDENT, Val: "C"},
			},
		},
		{
			Name: "TypeDeclWithFields",
			Src: `type Test {
	a: A
	b(): B
	c(a: A, b: B): [C!]!
	d: D @a @b @c
	type: Type
}`,
			Items: []Item{
				{Typ: token.TYPE, Val: "type"},
				{Typ: token.IDENT, Val: "Test"},
				{Typ: token.LBRACE, Val: "{"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.LBRACK, Val: "["},
				{Typ: token.IDENT, Val: "C"},
				{Typ: token.NOT, Val: "!"},
				{Typ: token.RBRACK, Val: "]"},
				{Typ: token.NOT, Val: "!"},
				{Typ: token.IDENT, Val: "d"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "D"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.TYPE, Val: "type"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "Type"},
				{Typ: token.RBRACE, Val: "}"},
			},
		},
		{
			Name: "TypeDeclWithAll",
			Src: `type Test implements A & B & C @a @b() @c(a: 1, b: "2", c: [1, 2, 3]) {
	a: A
	b(): B
	c(a: A, b: B): [C!]!
	d: D @a @b @c
	type: Type
}`,
			Items: []Item{
				{Typ: token.TYPE, Val: "type"},
				{Typ: token.IDENT, Val: "Test"},
				{Typ: token.IMPLEMENTS, Val: "implements"},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.IDENT, Val: "C"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.INT, Val: "1"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.STRING, Val: "\"2\""},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.LBRACK, Val: "["},
				{Typ: token.INT, Val: "1"},
				{Typ: token.INT, Val: "2"},
				{Typ: token.INT, Val: "3"},
				{Typ: token.RBRACK, Val: "]"},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.LBRACE, Val: "{"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.LBRACK, Val: "["},
				{Typ: token.IDENT, Val: "C"},
				{Typ: token.NOT, Val: "!"},
				{Typ: token.RBRACK, Val: "]"},
				{Typ: token.NOT, Val: "!"},
				{Typ: token.IDENT, Val: "d"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "D"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.TYPE, Val: "type"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "Type"},
				{Typ: token.RBRACE, Val: "}"},
			},
		},
		{
			Name: "Union",
			Src:  `union Test = A | B | C`,
			Items: []Item{
				{Typ: token.UNION, Val: "union"},
				{Typ: token.IDENT, Val: "Test"},
				{Typ: token.ASSIGN, Val: "="},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.IDENT, Val: "C"},
			},
		},
		{
			Name: "UnionWithDirectives",
			Src:  `union Test @a @b() @c(a: 1, b: "2", c: [1, 2, 3]) = A | B | C`,
			Items: []Item{
				{Typ: token.UNION, Val: "union"},
				{Typ: token.IDENT, Val: "Test"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.INT, Val: "1"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.STRING, Val: "\"2\""},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.LBRACK, Val: "["},
				{Typ: token.INT, Val: "1"},
				{Typ: token.INT, Val: "2"},
				{Typ: token.INT, Val: "3"},
				{Typ: token.RBRACK, Val: "]"},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.ASSIGN, Val: "="},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.IDENT, Val: "C"},
			},
		},
		{
			Name: "Enum",
			Src: `enum Test {
	A
	B
	C
}`,
			Items: []Item{
				{Typ: token.ENUM, Val: "enum"},
				{Typ: token.IDENT, Val: "Test"},
				{Typ: token.LBRACE, Val: "{"},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.IDENT, Val: "C"},
				{Typ: token.RBRACE, Val: "}"},
			},
		},
		{
			Name: "EnumValuesWithDirectives",
			Src: `enum Test {
	A @a
	B @a @b
	C @a @b() @c(a: 1, b: "2", c: [1, 2, 3])
}`,
			Items: []Item{
				{Typ: token.ENUM, Val: "enum"},
				{Typ: token.IDENT, Val: "Test"},
				{Typ: token.LBRACE, Val: "{"},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.IDENT, Val: "C"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.INT, Val: "1"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.STRING, Val: "\"2\""},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.LBRACK, Val: "["},
				{Typ: token.INT, Val: "1"},
				{Typ: token.INT, Val: "2"},
				{Typ: token.INT, Val: "3"},
				{Typ: token.RBRACK, Val: "]"},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.RBRACE, Val: "}"},
			},
		},
		{
			Name: "Input",
			Src: `input Test {
	a: A = 1
}`,
			Items: []Item{
				{Typ: token.INPUT, Val: "input"},
				{Typ: token.IDENT, Val: "Test"},
				{Typ: token.LBRACE, Val: "{"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.ASSIGN, Val: "="},
				{Typ: token.INT, Val: "1"},
				{Typ: token.RBRACE, Val: "}"},
			},
		},
		{
			Name: "Directive",
			Src:  `directive @test on A | B | C`,
			Items: []Item{
				{Typ: token.DIRECTIVE, Val: "directive"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "test"},
				{Typ: token.ON, Val: "on"},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.IDENT, Val: "C"},
			},
		},
		{
			Name: "DirectiveWithArgs",
			Src:  `directive @test(a: A = 1 @a, b: B @a @b, c: C @c(b: {hello: "world!"})) on A | B | C`,
			Items: []Item{
				{Typ: token.DIRECTIVE, Val: "directive"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "test"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.ASSIGN, Val: "="},
				{Typ: token.INT, Val: "1"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.IDENT, Val: "C"},
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "c"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.IDENT, Val: "b"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.LBRACE, Val: "{"},
				{Typ: token.IDENT, Val: "hello"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.STRING, Val: "\"world!\""},
				{Typ: token.RBRACE, Val: "}"},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.RPAREN, Val: ")"},
				{Typ: token.ON, Val: "on"},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.IDENT, Val: "B"},
				{Typ: token.IDENT, Val: "C"},
			},
		},
		{
			Name: "ExtendSchema",
			Src:  `extend schema {}`,
			Items: []Item{
				{Typ: token.EXTEND, Val: "extend"},
				{Typ: token.SCHEMA, Val: "schema"},
				{Typ: token.LBRACE, Val: "{"},
				{Typ: token.RBRACE, Val: "}"},
			},
		},
		{
			Name: "ExtendType",
			Src:  `extend scalar Test`,
			Items: []Item{
				{Typ: token.EXTEND, Val: "extend"},
				{Typ: token.SCALAR, Val: "scalar"},
				{Typ: token.IDENT, Val: "Test"},
			},
		},
		{
			Name: "WithDocumentation",
			Src: `# Comment

"Single Above"
"""
Multi above
"""
"Before" enum Test {
	# Field comment lvl
	"A"
	A
}`,
			Items: []Item{
				{Typ: token.COMMENT, Val: "# Comment\n"},
				{Typ: token.DESCRIPTION, Val: `"Single Above"`},
				{Typ: token.DESCRIPTION, Val: `"""
Multi above
"""`},
				{Typ: token.DESCRIPTION, Val: `"Before"`},
				{Typ: token.ENUM, Val: "enum"},
				{Typ: token.IDENT, Val: "Test"},
				{Typ: token.LBRACE, Val: "{"},
				{Typ: token.COMMENT, Val: "# Field comment lvl\n"},
				{Typ: token.DESCRIPTION, Val: `"A"`},
				{Typ: token.IDENT, Val: "A"},
				{Typ: token.RBRACE, Val: "}"},
			},
		},
		{
			Name: "TopLvlDirective",
			Src:  `@import(paths: ["hello"])`,
			Items: []Item{
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "import"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.IDENT, Val: "paths"},
				{Typ: token.COLON, Val: ":"},
				{Typ: token.LBRACK, Val: "["},
				{Typ: token.STRING, Val: "\"hello\""},
				{Typ: token.RBRACK, Val: "]"},
				{Typ: token.RPAREN, Val: ")"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			dset := token.NewDocSet()

			l := Lex(dset.AddDoc("", dset.Base(), len(testCase.Src)), testCase.Src)

			expectItems(subT, l, testCase.Items...)
			expectEOF(subT, l)
		})
	}
}

func TestErrs(t *testing.T) {
	testCases := []struct {
		Name  string
		Src   string
		Items []Item
	}{
		{
			Name: "MalformedDescription",
			Src:  `"hello`,
			Items: []Item{
				{Typ: token.ERR, Val: "bad string syntax: \"hello"},
			},
		},
		{
			Name: "MalformedDirectiveArgs",
			Src:  `@test(`,
			Items: []Item{
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "test"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.ERR, Val: "invalid arg"},
			},
		},
		{
			Name: "MalformedDirectiveArgs2",
			Src:  `@test(a  1)`,
			Items: []Item{
				{Typ: token.AT, Val: "@"},
				{Typ: token.IDENT, Val: "test"},
				{Typ: token.LPAREN, Val: "("},
				{Typ: token.IDENT, Val: "a"},
				{Typ: token.ERR, Val: "invalid arg"},
			},
		},
		{
			Name: "UnknownTypeDecl",
			Src:  `unknownType Test`,
			Items: []Item{
				{Typ: token.ERR, Val: "invalid type declaration"},
			},
		},
		{
			Name: "InvalidTypeExtension",
			Src:  `extend unknownType Test`,
			Items: []Item{
				{Typ: token.EXTEND, Val: "extend"},
				{Typ: token.ERR, Val: "invalid type extension"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			dset := token.NewDocSet()

			l := Lex(dset.AddDoc("", dset.Base(), len(testCase.Src)), testCase.Src)

			expectItems(subT, l, testCase.Items...)
			l.Drain()
		})
	}
}

func TestLex(t *testing.T) {
	dset := token.NewDocSet()
	l := Lex(dset.AddDoc("", dset.Base(), len(gqlSrc)), string(gqlSrc))

	expectItems(t, l, testItems.Items...)
	expectEOF(t, l)
}

func BenchmarkLex(b *testing.B) {
	benchSrcStr := string(gqlSrc)

	d := token.NewDocSet().AddDoc("test", -1, len(benchSrcStr))
	for i := 0; i < b.N; i++ {
		l := Lex(d, benchSrcStr)
		l.Drain()
	}
}
