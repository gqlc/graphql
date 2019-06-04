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

	update bool
)

func init() {
	flag.StringVar(&gqlFile, "gqlFile", "test.gql", "Specify a .gql file for testing/benchmarking")
	flag.StringVar(&testItemsFile, "itemsFile", "items.json", "Specify a .json file that contains the lexed items for the .gql file")

	flag.BoolVar(&update, "update", false, "Update itemsFile")
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

func TestUpdate(t *testing.T) {
	if !update {
		t.Skipf("not updating items file: %s", testItemsFile)
		return
	}

	dset := token.NewDocSet()
	l := Lex(dset.AddDoc(gqlFile, dset.Base(), len(gqlSrc)), string(gqlSrc))

	var items []Item
	for {
		item := l.NextItem()
		if item.Typ == token.Token_ERR {
			t.Error(item)
			return
		}
		if item.Typ == token.Token_EOF {
			break
		}

		items = append(items, item)
	}

	f, err := os.OpenFile(testItemsFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		t.Error(err)
		return
	}

	enc := json.NewEncoder(f)
	err = enc.Encode(struct {
		Items []Item `json:"items"`
	}{Items: items})
	if err != nil {
		t.Error(err)
		return
	}
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
	if i.Typ != token.Token_EOF {
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
				{Typ: token.Token_SCHEMA, Val: "schema"},
				{Typ: token.Token_LBRACE, Val: "{"},
				{Typ: token.Token_RBRACE, Val: "}"},
			},
		},
		{
			Name: "SchemaWithDirectives",
			Src:  `schema @a @b() @c(a: 1, b: "2", c: [1, 2, 3]) {}`,
			Items: []Item{
				{Typ: token.Token_SCHEMA, Val: "schema"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_INT, Val: "1"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_STRING, Val: "\"2\""},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_LBRACK, Val: "["},
				{Typ: token.Token_INT, Val: "1"},
				{Typ: token.Token_INT, Val: "2"},
				{Typ: token.Token_INT, Val: "3"},
				{Typ: token.Token_RBRACK, Val: "]"},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_LBRACE, Val: "{"},
				{Typ: token.Token_RBRACE, Val: "}"},
			},
		},
		{
			Name: "TypeDecl",
			Src:  `scalar Test`,
			Items: []Item{
				{Typ: token.Token_SCALAR, Val: "scalar"},
				{Typ: token.Token_IDENT, Val: "Test"},
			},
		},
		{
			Name: "TypeDeclWithDirectives",
			Src:  `scalar Test @a @b() @c(a: 1, b: "2", c: [1, 2, 3])`,
			Items: []Item{
				{Typ: token.Token_SCALAR, Val: "scalar"},
				{Typ: token.Token_IDENT, Val: "Test"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_INT, Val: "1"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_STRING, Val: "\"2\""},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_LBRACK, Val: "["},
				{Typ: token.Token_INT, Val: "1"},
				{Typ: token.Token_INT, Val: "2"},
				{Typ: token.Token_INT, Val: "3"},
				{Typ: token.Token_RBRACK, Val: "]"},
				{Typ: token.Token_RPAREN, Val: ")"},
			},
		},
		{
			Name: "TypeDeclWithInterfaces",
			Src:  `type Test implements A & B & C`,
			Items: []Item{
				{Typ: token.Token_TYPE, Val: "type"},
				{Typ: token.Token_IDENT, Val: "Test"},
				{Typ: token.Token_IMPLEMENTS, Val: "implements"},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_AND, Val: "&"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_AND, Val: "&"},
				{Typ: token.Token_IDENT, Val: "C"},
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
				{Typ: token.Token_TYPE, Val: "type"},
				{Typ: token.Token_IDENT, Val: "Test"},
				{Typ: token.Token_LBRACE, Val: "{"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_LBRACK, Val: "["},
				{Typ: token.Token_IDENT, Val: "C"},
				{Typ: token.Token_NOT, Val: "!"},
				{Typ: token.Token_RBRACK, Val: "]"},
				{Typ: token.Token_NOT, Val: "!"},
				{Typ: token.Token_IDENT, Val: "d"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "D"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_TYPE, Val: "type"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "Type"},
				{Typ: token.Token_RBRACE, Val: "}"},
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
				{Typ: token.Token_TYPE, Val: "type"},
				{Typ: token.Token_IDENT, Val: "Test"},
				{Typ: token.Token_IMPLEMENTS, Val: "implements"},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_AND, Val: "&"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_AND, Val: "&"},
				{Typ: token.Token_IDENT, Val: "C"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_INT, Val: "1"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_STRING, Val: "\"2\""},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_LBRACK, Val: "["},
				{Typ: token.Token_INT, Val: "1"},
				{Typ: token.Token_INT, Val: "2"},
				{Typ: token.Token_INT, Val: "3"},
				{Typ: token.Token_RBRACK, Val: "]"},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_LBRACE, Val: "{"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_LBRACK, Val: "["},
				{Typ: token.Token_IDENT, Val: "C"},
				{Typ: token.Token_NOT, Val: "!"},
				{Typ: token.Token_RBRACK, Val: "]"},
				{Typ: token.Token_NOT, Val: "!"},
				{Typ: token.Token_IDENT, Val: "d"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "D"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_TYPE, Val: "type"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "Type"},
				{Typ: token.Token_RBRACE, Val: "}"},
			},
		},
		{
			Name: "Union",
			Src:  `union Test = A | B | C`,
			Items: []Item{
				{Typ: token.Token_UNION, Val: "union"},
				{Typ: token.Token_IDENT, Val: "Test"},
				{Typ: token.Token_ASSIGN, Val: "="},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_OR, Val: "|"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_OR, Val: "|"},
				{Typ: token.Token_IDENT, Val: "C"},
			},
		},
		{
			Name: "UnionWithDirectives",
			Src:  `union Test @a @b() @c(a: 1, b: "2", c: [1, 2, 3]) = A | B | C`,
			Items: []Item{
				{Typ: token.Token_UNION, Val: "union"},
				{Typ: token.Token_IDENT, Val: "Test"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_INT, Val: "1"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_STRING, Val: "\"2\""},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_LBRACK, Val: "["},
				{Typ: token.Token_INT, Val: "1"},
				{Typ: token.Token_INT, Val: "2"},
				{Typ: token.Token_INT, Val: "3"},
				{Typ: token.Token_RBRACK, Val: "]"},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_ASSIGN, Val: "="},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_OR, Val: "|"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_OR, Val: "|"},
				{Typ: token.Token_IDENT, Val: "C"},
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
				{Typ: token.Token_ENUM, Val: "enum"},
				{Typ: token.Token_IDENT, Val: "Test"},
				{Typ: token.Token_LBRACE, Val: "{"},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_IDENT, Val: "C"},
				{Typ: token.Token_RBRACE, Val: "}"},
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
				{Typ: token.Token_ENUM, Val: "enum"},
				{Typ: token.Token_IDENT, Val: "Test"},
				{Typ: token.Token_LBRACE, Val: "{"},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_IDENT, Val: "C"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_INT, Val: "1"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_STRING, Val: "\"2\""},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_LBRACK, Val: "["},
				{Typ: token.Token_INT, Val: "1"},
				{Typ: token.Token_INT, Val: "2"},
				{Typ: token.Token_INT, Val: "3"},
				{Typ: token.Token_RBRACK, Val: "]"},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_RBRACE, Val: "}"},
			},
		},
		{
			Name: "Input",
			Src: `input Test {
	a: A = 1
}`,
			Items: []Item{
				{Typ: token.Token_INPUT, Val: "input"},
				{Typ: token.Token_IDENT, Val: "Test"},
				{Typ: token.Token_LBRACE, Val: "{"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_ASSIGN, Val: "="},
				{Typ: token.Token_INT, Val: "1"},
				{Typ: token.Token_RBRACE, Val: "}"},
			},
		},
		{
			Name: "Directive",
			Src:  `directive @test on A | B | C`,
			Items: []Item{
				{Typ: token.Token_DIRECTIVE, Val: "directive"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "test"},
				{Typ: token.Token_ON, Val: "on"},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_OR, Val: "|"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_OR, Val: "|"},
				{Typ: token.Token_IDENT, Val: "C"},
			},
		},
		{
			Name: "DirectiveWithArgs",
			Src:  `directive @test(a: A = 1 @a, b: B @a @b, c: C @c(b: {hello: "world!"})) on A | B | C`,
			Items: []Item{
				{Typ: token.Token_DIRECTIVE, Val: "directive"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "test"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_ASSIGN, Val: "="},
				{Typ: token.Token_INT, Val: "1"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_IDENT, Val: "C"},
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "c"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_IDENT, Val: "b"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_LBRACE, Val: "{"},
				{Typ: token.Token_IDENT, Val: "hello"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_STRING, Val: "\"world!\""},
				{Typ: token.Token_RBRACE, Val: "}"},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_RPAREN, Val: ")"},
				{Typ: token.Token_ON, Val: "on"},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_OR, Val: "|"},
				{Typ: token.Token_IDENT, Val: "B"},
				{Typ: token.Token_OR, Val: "|"},
				{Typ: token.Token_IDENT, Val: "C"},
			},
		},
		{
			Name: "ExtendSchema",
			Src:  `extend schema {}`,
			Items: []Item{
				{Typ: token.Token_EXTEND, Val: "extend"},
				{Typ: token.Token_SCHEMA, Val: "schema"},
				{Typ: token.Token_LBRACE, Val: "{"},
				{Typ: token.Token_RBRACE, Val: "}"},
			},
		},
		{
			Name: "ExtendType",
			Src:  `extend scalar Test`,
			Items: []Item{
				{Typ: token.Token_EXTEND, Val: "extend"},
				{Typ: token.Token_SCALAR, Val: "scalar"},
				{Typ: token.Token_IDENT, Val: "Test"},
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
				{Typ: token.Token_COMMENT, Val: "# Comment\n"},
				{Typ: token.Token_DESCRIPTION, Val: `"Single Above"`},
				{Typ: token.Token_DESCRIPTION, Val: `"""
Multi above
"""`},
				{Typ: token.Token_DESCRIPTION, Val: `"Before"`},
				{Typ: token.Token_ENUM, Val: "enum"},
				{Typ: token.Token_IDENT, Val: "Test"},
				{Typ: token.Token_LBRACE, Val: "{"},
				{Typ: token.Token_COMMENT, Val: "# Field comment lvl\n"},
				{Typ: token.Token_DESCRIPTION, Val: `"A"`},
				{Typ: token.Token_IDENT, Val: "A"},
				{Typ: token.Token_RBRACE, Val: "}"},
			},
		},
		{
			Name: "TopLvlDirective",
			Src:  `@import(paths: ["hello"])`,
			Items: []Item{
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "import"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_IDENT, Val: "paths"},
				{Typ: token.Token_COLON, Val: ":"},
				{Typ: token.Token_LBRACK, Val: "["},
				{Typ: token.Token_STRING, Val: "\"hello\""},
				{Typ: token.Token_RBRACK, Val: "]"},
				{Typ: token.Token_RPAREN, Val: ")"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			dset := token.NewDocSet()

			l := Lex(dset.AddDoc(testCase.Name, dset.Base(), len(testCase.Src)), testCase.Src)

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
				{Typ: token.Token_ERR, Val: "bad string syntax: \"hello"},
			},
		},
		{
			Name: "MalformedDirectiveArgs",
			Src:  `@test(`,
			Items: []Item{
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "test"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_ERR, Val: "invalid arg"},
			},
		},
		{
			Name: "MalformedDirectiveArgs2",
			Src:  `@test(a  1)`,
			Items: []Item{
				{Typ: token.Token_AT, Val: "@"},
				{Typ: token.Token_IDENT, Val: "test"},
				{Typ: token.Token_LPAREN, Val: "("},
				{Typ: token.Token_IDENT, Val: "a"},
				{Typ: token.Token_ERR, Val: "invalid arg"},
			},
		},
		{
			Name: "UnknownTypeDecl",
			Src:  `unknownType Test`,
			Items: []Item{
				{Typ: token.Token_ERR, Val: "invalid type declaration"},
			},
		},
		{
			Name: "InvalidTypeExtension",
			Src:  `extend unknownType Test`,
			Items: []Item{
				{Typ: token.Token_EXTEND, Val: "extend"},
				{Typ: token.Token_ERR, Val: "invalid type extension"},
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
