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

func TestLexDocDirectives(t *testing.T) {
	fset := token.NewDocSet()
	src := []byte(`@test

@test
scalar URI
@test`)
	l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
	expectItems(t, l, []Item{
		{Typ: token.AT, Line: 1, Pos: 1, Val: "@"},
		{Typ: token.IDENT, Line: 1, Pos: 2, Val: "test"},
		{Typ: token.AT, Line: 3, Pos: 8, Val: "@"},
		{Typ: token.IDENT, Line: 3, Pos: 9, Val: "test"},
		{Typ: token.SCALAR, Line: 4, Pos: 14, Val: "scalar"},
		{Typ: token.IDENT, Line: 4, Pos: 21, Val: "URI"},
		{Typ: token.AT, Line: 5, Pos: 25, Val: "@"},
		{Typ: token.IDENT, Line: 5, Pos: 26, Val: "test"},
	}...)
}

func TestLexScalar(t *testing.T) {

	t.Run("Simple", func(subT *testing.T) {
		fset := token.NewDocSet()
		src := []byte(`scalar URI`)
		l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
		expectItems(subT, l, []Item{
			{Typ: token.SCALAR, Line: 1, Pos: 1, Val: "scalar"},
			{Typ: token.IDENT, Line: 1, Pos: 8, Val: "URI"},
		}...)
		expectEOF(subT, l)
	})

	t.Run("WithDirectives", func(subT *testing.T) {
		fset := token.NewDocSet()
		src := []byte(`scalar URI @gotype @jstype() @darttype(if: Boolean)`)
		l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
		expectItems(subT, l, []Item{
			{Typ: token.SCALAR, Line: 1, Pos: 1, Val: "scalar"},
			{Typ: token.IDENT, Line: 1, Pos: 8, Val: "URI"},
			{Typ: token.AT, Line: 1, Pos: 12, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 13, Val: "gotype"},
			{Typ: token.AT, Line: 1, Pos: 20, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 21, Val: "jstype"},
			{Typ: token.LPAREN, Line: 1, Pos: 27, Val: "("},
			{Typ: token.RPAREN, Line: 1, Pos: 28, Val: ")"},
			{Typ: token.AT, Line: 1, Pos: 30, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 31, Val: "darttype"},
			{Typ: token.LPAREN, Line: 1, Pos: 39, Val: "("},
			{Typ: token.IDENT, Line: 1, Pos: 40, Val: "if"},
			{Typ: token.COLON, Line: 1, Pos: 42, Val: ":"},
			{Typ: token.IDENT, Line: 1, Pos: 44, Val: "Boolean"},
			{Typ: token.RPAREN, Line: 1, Pos: 51, Val: ")"},
		}...)
		expectEOF(subT, l)
	})
}

func TestScanValue(t *testing.T) {
	// Note:
	//    - Boolean, Null, and Enum values are all IDENT tokens so
	// 		they don't need to be tested here
	//	  - Strings are tested by LexImports
	//    - List and Objects are handled by ScanList which is implemented in others

	t.Run("Var", func(subT *testing.T) {
		fset := token.NewDocSet()
		src := []byte(`$a`)
		l := &lxr{
			line:  1,
			items: make(chan Item),
			src:   src,
			doc:   fset.AddDoc("", fset.Base(), len(src)),
		}

		go func() {
			l.scanValue()
			close(l.items)
		}()

		expectItems(subT, l, []Item{
			{Typ: token.VAR, Line: 1, Pos: 1, Val: "$"},
			{Typ: token.IDENT, Line: 1, Pos: 2, Val: "a"},
		}...)
	})

	t.Run("Int", func(subT *testing.T) {
		fset := token.NewDocSet()
		src := []byte(`12354654684013246813216513213254686210`)
		l := &lxr{
			line:  1,
			items: make(chan Item),
			src:   src,
			doc:   fset.AddDoc("", fset.Base(), len(src)),
		}

		go func() {
			l.scanValue()
			close(l.items)
		}()

		expectItems(subT, l,
			Item{Typ: token.INT, Line: 1, Pos: 1, Val: "12354654684013246813216513213254686210"},
		)
	})

	t.Run("Float", func(subT *testing.T) {

		subT.Run("fractional", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`123.45`)
			l := &lxr{
				line:  1,
				items: make(chan Item),
				src:   src,
				doc:   fset.AddDoc("", fset.Base(), len(src)),
			}

			go func() {
				l.scanValue()
				close(l.items)
			}()

			expectItems(subT, l,
				Item{Typ: token.FLOAT, Line: 1, Pos: 1, Val: "123.45"},
			)
		})

		subT.Run("exponential", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`123e45`)
			l := &lxr{
				line:  1,
				items: make(chan Item),
				src:   src,
				doc:   fset.AddDoc("", fset.Base(), len(src)),
			}

			go func() {
				l.scanValue()
				close(l.items)
			}()

			expectItems(subT, l,
				Item{Typ: token.FLOAT, Line: 1, Pos: 1, Val: "123e45"},
			)
		})

		subT.Run("full", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`123.45e6`)
			l := &lxr{
				line:  1,
				items: make(chan Item),
				src:   src,
				doc:   fset.AddDoc("", fset.Base(), len(src)),
			}

			go func() {
				l.scanValue()
				close(l.items)
			}()

			expectItems(subT, l,
				Item{Typ: token.FLOAT, Line: 1, Pos: 1, Val: "123.45e6"},
			)
		})
	})

	t.Run("Composite", func(subT *testing.T) {

		subT.Run("List", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`["1", 1, true, 1.2]`)
			l := &lxr{
				line:  1,
				items: make(chan Item),
				src:   src,
				doc:   fset.AddDoc("", fset.Base(), len(src)),
			}

			go func() {
				l.scanValue()
				close(l.items)
			}()

			expectItems(triT, l,
				Item{Typ: token.LBRACK, Line: 1, Pos: 1, Val: "["},
				Item{Typ: token.STRING, Line: 1, Pos: 2, Val: `"1"`},
				Item{Typ: token.INT, Line: 1, Pos: 7, Val: "1"},
				Item{Typ: token.IDENT, Line: 1, Pos: 10, Val: "true"},
				Item{Typ: token.FLOAT, Line: 1, Pos: 16, Val: "1.2"},
				Item{Typ: token.RBRACK, Line: 1, Pos: 19, Val: "]"},
			)
		})

		subT.Run("Object", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`{hello: "world", one: 1, two: 2.5, thr: true}`)
			l := &lxr{
				line:  1,
				items: make(chan Item),
				src:   src,
				doc:   fset.AddDoc("", fset.Base(), len(src)),
			}

			go func() {
				l.scanValue()
				close(l.items)
			}()

			expectItems(triT, l,
				Item{Typ: token.LBRACE, Line: 1, Pos: 1, Val: "{"},
				Item{Typ: token.IDENT, Line: 1, Pos: 2, Val: "hello"},
				Item{Typ: token.COLON, Line: 1, Pos: 7, Val: ":"},
				Item{Typ: token.STRING, Line: 1, Pos: 9, Val: `"world"`},
				Item{Typ: token.IDENT, Line: 1, Pos: 18, Val: "one"},
				Item{Typ: token.COLON, Line: 1, Pos: 21, Val: ":"},
				Item{Typ: token.INT, Line: 1, Pos: 23, Val: "1"},
				Item{Typ: token.IDENT, Line: 1, Pos: 26, Val: "two"},
				Item{Typ: token.COLON, Line: 1, Pos: 29, Val: ":"},
				Item{Typ: token.FLOAT, Line: 1, Pos: 31, Val: "2.5"},
				Item{Typ: token.IDENT, Line: 1, Pos: 36, Val: "thr"},
				Item{Typ: token.COLON, Line: 1, Pos: 39, Val: ":"},
				Item{Typ: token.IDENT, Line: 1, Pos: 41, Val: "true"},
				Item{Typ: token.RBRACE, Line: 1, Pos: 45, Val: "}"},
			)
		})
	})
}

func TestLexObject(t *testing.T) {

	t.Run("WithImpls", func(subT *testing.T) {

		subT.Run("Perfect", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`type Rect implements One & Two & Three & Four`)
			l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
			expectItems(triT, l, []Item{
				{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
				{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
				{Typ: token.IMPLEMENTS, Line: 1, Pos: 11, Val: "implements"},
				{Typ: token.IDENT, Line: 1, Pos: 22, Val: "One"},
				{Typ: token.IDENT, Line: 1, Pos: 28, Val: "Two"},
				{Typ: token.IDENT, Line: 1, Pos: 34, Val: "Three"},
				{Typ: token.IDENT, Line: 1, Pos: 42, Val: "Four"},
			}...)
			expectEOF(triT, l)
		})

		subT.Run("InvalidSeparator", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`type Rect implements One , Two & Three`)
			l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
			expectItems(triT, l, []Item{
				{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
				{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
				{Typ: token.IMPLEMENTS, Line: 1, Pos: 11, Val: "implements"},
				{Typ: token.IDENT, Line: 1, Pos: 22, Val: "One"},
				{Typ: token.ERR, Line: 1, Pos: 26, Val: "invalid list separator: 44"},
			}...)

			fset = token.NewDocSet()
			src = []byte(`type Rect implements One & Two , Three`)
			l = Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
			expectItems(triT, l, []Item{
				{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
				{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
				{Typ: token.IMPLEMENTS, Line: 1, Pos: 11, Val: "implements"},
				{Typ: token.IDENT, Line: 1, Pos: 22, Val: "One"},
				{Typ: token.IDENT, Line: 1, Pos: 28, Val: "Two"},
				{Typ: token.ERR, Line: 1, Pos: 32, Val: "invalid list separator: 44"},
			}...)
		})
	})

	t.Run("WithDirectives", func(subT *testing.T) {

		subT.Run("EndsWithBrace", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`type Rect @green @blue {}`)
			l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
			expectItems(triT, l, []Item{
				{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
				{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
				{Typ: token.AT, Line: 1, Pos: 11, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 12, Val: "green"},
				{Typ: token.AT, Line: 1, Pos: 18, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 19, Val: "blue"},
				{Typ: token.LBRACE, Line: 1, Pos: 24, Val: "{"},
				{Typ: token.RBRACE, Line: 1, Pos: 25, Val: "}"},
			}...)
			expectEOF(triT, l)
		})

		subT.Run("EndsWithNewline", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`type Rect @green @blue
`)
			l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
			expectItems(triT, l, []Item{
				{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
				{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
				{Typ: token.AT, Line: 1, Pos: 11, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 12, Val: "green"},
				{Typ: token.AT, Line: 1, Pos: 18, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 19, Val: "blue"},
			}...)
			expectEOF(triT, l)
		})

		subT.Run("EndsWithEOF", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`type Rect @green @blue`)
			l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
			expectItems(triT, l, []Item{
				{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
				{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
				{Typ: token.AT, Line: 1, Pos: 11, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 12, Val: "green"},
				{Typ: token.AT, Line: 1, Pos: 18, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 19, Val: "blue"},
			}...)
			expectEOF(triT, l)
		})
	})

	t.Run("WithImpls&Directives", func(subT *testing.T) {

		subT.Run("EndsWithBrace", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`type Rect implements One & Two & Three @green @blue {}`)
			l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
			expectItems(triT, l, []Item{
				{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
				{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
				{Typ: token.IMPLEMENTS, Line: 1, Pos: 11, Val: "implements"},
				{Typ: token.IDENT, Line: 1, Pos: 22, Val: "One"},
				{Typ: token.IDENT, Line: 1, Pos: 28, Val: "Two"},
				{Typ: token.IDENT, Line: 1, Pos: 34, Val: "Three"},
				{Typ: token.AT, Line: 1, Pos: 40, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 41, Val: "green"},
				{Typ: token.AT, Line: 1, Pos: 47, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 48, Val: "blue"},
				{Typ: token.LBRACE, Line: 1, Pos: 53, Val: "{"},
				{Typ: token.RBRACE, Line: 1, Pos: 54, Val: "}"},
			}...)
			expectEOF(triT, l)
		})

		subT.Run("EndsWithNewline", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`type Rect implements One & Two & Three @green @blue
`)
			l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
			expectItems(triT, l, []Item{
				{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
				{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
				{Typ: token.IMPLEMENTS, Line: 1, Pos: 11, Val: "implements"},
				{Typ: token.IDENT, Line: 1, Pos: 22, Val: "One"},
				{Typ: token.IDENT, Line: 1, Pos: 28, Val: "Two"},
				{Typ: token.IDENT, Line: 1, Pos: 34, Val: "Three"},
				{Typ: token.AT, Line: 1, Pos: 40, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 41, Val: "green"},
				{Typ: token.AT, Line: 1, Pos: 47, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 48, Val: "blue"},
			}...)
			expectEOF(triT, l)
		})

		subT.Run("EndsWithEOF", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`type Rect implements One & Two & Three @green @blue`)
			l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
			expectItems(triT, l, []Item{
				{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
				{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
				{Typ: token.IMPLEMENTS, Line: 1, Pos: 11, Val: "implements"},
				{Typ: token.IDENT, Line: 1, Pos: 22, Val: "One"},
				{Typ: token.IDENT, Line: 1, Pos: 28, Val: "Two"},
				{Typ: token.IDENT, Line: 1, Pos: 34, Val: "Three"},
				{Typ: token.AT, Line: 1, Pos: 40, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 41, Val: "green"},
				{Typ: token.AT, Line: 1, Pos: 47, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 48, Val: "blue"},
			}...)
			expectEOF(triT, l)
		})
	})

	t.Run("WithFields", func(subT *testing.T) {

		subT.Run("AsFieldsDef", func(triT *testing.T) {

			triT.Run("simple", func(qt *testing.T) {
				fset := token.NewDocSet()
				src := []byte(`type Rect {
	one: One
	two: Two
}`)
				l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
				expectItems(qt, l, []Item{
					{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
					{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
					{Typ: token.LBRACE, Line: 1, Pos: 11, Val: "{"},
					{Typ: token.IDENT, Line: 2, Pos: 14, Val: "one"},
					{Typ: token.COLON, Line: 2, Pos: 17, Val: ":"},
					{Typ: token.IDENT, Line: 2, Pos: 19, Val: "One"},
					{Typ: token.IDENT, Line: 3, Pos: 24, Val: "two"},
					{Typ: token.COLON, Line: 3, Pos: 27, Val: ":"},
					{Typ: token.IDENT, Line: 3, Pos: 29, Val: "Two"},
					{Typ: token.RBRACE, Line: 4, Pos: 33, Val: "}"},
				}...)
				expectEOF(qt, l)
			})

			triT.Run("withDescrs", func(qt *testing.T) {
				fset := token.NewDocSet()
				src := []byte(`type Rect {
	"one descr" one: One
	"""
	two descr
	"""
	two: Two
}`)
				l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
				expectItems(qt, l, []Item{
					{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
					{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
					{Typ: token.LBRACE, Line: 1, Pos: 11, Val: "{"},
					{Typ: token.DESCRIPTION, Line: 2, Pos: 14, Val: `"one descr"`},
					{Typ: token.IDENT, Line: 2, Pos: 26, Val: "one"},
					{Typ: token.COLON, Line: 2, Pos: 29, Val: ":"},
					{Typ: token.IDENT, Line: 2, Pos: 31, Val: "One"},
					{Typ: token.DESCRIPTION, Line: 5, Pos: 36, Val: "\"\"\"\n\ttwo descr\n\t\"\"\""},
					{Typ: token.IDENT, Line: 6, Pos: 57, Val: "two"},
					{Typ: token.COLON, Line: 6, Pos: 60, Val: ":"},
					{Typ: token.IDENT, Line: 6, Pos: 62, Val: "Two"},
					{Typ: token.RBRACE, Line: 7, Pos: 66, Val: "}"},
				}...)
				expectEOF(qt, l)
			})

			triT.Run("withArgs", func(qt *testing.T) {
				fset := token.NewDocSet()
				src := []byte(`type Rect {
	one(a: A, b: B): One
	two(
	"a descr" a: A
	"""
	b descr
	"""
	b: B
): Two
}`)
				l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
				expectItems(qt, l, []Item{
					{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
					{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
					{Typ: token.LBRACE, Line: 1, Pos: 11, Val: "{"},
					{Typ: token.IDENT, Line: 2, Pos: 14, Val: "one"},
					{Typ: token.LPAREN, Line: 2, Pos: 17, Val: "("},
					{Typ: token.IDENT, Line: 2, Pos: 18, Val: "a"},
					{Typ: token.COLON, Line: 2, Pos: 19, Val: ":"},
					{Typ: token.IDENT, Line: 2, Pos: 21, Val: "A"},
					{Typ: token.IDENT, Line: 2, Pos: 24, Val: "b"},
					{Typ: token.COLON, Line: 2, Pos: 25, Val: ":"},
					{Typ: token.IDENT, Line: 2, Pos: 27, Val: "B"},
					{Typ: token.RPAREN, Line: 2, Pos: 28, Val: ")"},
					{Typ: token.COLON, Line: 2, Pos: 29, Val: ":"},
					{Typ: token.IDENT, Line: 2, Pos: 31, Val: "One"},
					{Typ: token.IDENT, Line: 3, Pos: 36, Val: "two"},
					{Typ: token.LPAREN, Line: 3, Pos: 39, Val: "("},
					{Typ: token.DESCRIPTION, Line: 4, Pos: 42, Val: `"a descr"`},
					{Typ: token.IDENT, Line: 4, Pos: 52, Val: "a"},
					{Typ: token.COLON, Line: 4, Pos: 53, Val: ":"},
					{Typ: token.IDENT, Line: 4, Pos: 55, Val: "A"},
					{Typ: token.DESCRIPTION, Line: 7, Pos: 58, Val: "\"\"\"\n\tb descr\n\t\"\"\""},
					{Typ: token.IDENT, Line: 8, Pos: 77, Val: "b"},
					{Typ: token.COLON, Line: 8, Pos: 78, Val: ":"},
					{Typ: token.IDENT, Line: 8, Pos: 80, Val: "B"},
					{Typ: token.RPAREN, Line: 9, Pos: 82, Val: ")"},
					{Typ: token.COLON, Line: 9, Pos: 83, Val: ":"},
					{Typ: token.IDENT, Line: 9, Pos: 85, Val: "Two"},
					{Typ: token.RBRACE, Line: 10, Pos: 89, Val: "}"},
				}...)
				expectEOF(qt, l)
			})

			triT.Run("withDirectives", func(qt *testing.T) {
				fset := token.NewDocSet()
				src := []byte(`type Rect {
	one: One @green @blue
	two: Two @blue
}`)
				l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
				expectItems(qt, l, []Item{
					{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
					{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
					{Typ: token.LBRACE, Line: 1, Pos: 11, Val: "{"},
					{Typ: token.IDENT, Line: 2, Pos: 14, Val: "one"},
					{Typ: token.COLON, Line: 2, Pos: 17, Val: ":"},
					{Typ: token.IDENT, Line: 2, Pos: 19, Val: "One"},
					{Typ: token.AT, Line: 2, Pos: 23, Val: "@"},
					{Typ: token.IDENT, Line: 2, Pos: 24, Val: "green"},
					{Typ: token.AT, Line: 2, Pos: 30, Val: "@"},
					{Typ: token.IDENT, Line: 2, Pos: 31, Val: "blue"},
					{Typ: token.IDENT, Line: 3, Pos: 37, Val: "two"},
					{Typ: token.COLON, Line: 3, Pos: 40, Val: ":"},
					{Typ: token.IDENT, Line: 3, Pos: 42, Val: "Two"},
					{Typ: token.AT, Line: 3, Pos: 46, Val: "@"},
					{Typ: token.IDENT, Line: 3, Pos: 47, Val: "blue"},
					{Typ: token.RBRACE, Line: 4, Pos: 52, Val: "}"},
				}...)
				expectEOF(qt, l)
			})
		})

		subT.Run("AsEnumValsDef", func(triT *testing.T) {

			triT.Run("simple", func(qt *testing.T) {
				fset := token.NewDocSet()
				src := []byte(`enum Rect {
	LEFT
	UP
	RIGHT
	DOWN
}`)
				l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
				expectItems(qt, l, []Item{
					{Typ: token.ENUM, Line: 1, Pos: 1, Val: "enum"},
					{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
					{Typ: token.LBRACE, Line: 1, Pos: 11, Val: "{"},
					{Typ: token.IDENT, Line: 2, Pos: 14, Val: "LEFT"},
					{Typ: token.IDENT, Line: 3, Pos: 20, Val: "UP"},
					{Typ: token.IDENT, Line: 4, Pos: 24, Val: "RIGHT"},
					{Typ: token.IDENT, Line: 5, Pos: 31, Val: "DOWN"},
					{Typ: token.RBRACE, Line: 6, Pos: 36, Val: "}"},
				}...)
				expectEOF(qt, l)
			})

			triT.Run("withDescrs", func(qt *testing.T) {
				fset := token.NewDocSet()
				src := []byte(`enum Rect {
	"left descr" LEFT
	"up descr" UP
	"""
	right descr
	"""
	RIGHT
	"down descr above"
	"down descr before" DOWN
}`)
				l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
				expectItems(qt, l, []Item{
					{Typ: token.ENUM, Line: 1, Pos: 1, Val: "enum"},
					{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
					{Typ: token.LBRACE, Line: 1, Pos: 11, Val: "{"},
					{Typ: token.DESCRIPTION, Line: 2, Pos: 14, Val: `"left descr"`},
					{Typ: token.IDENT, Line: 2, Pos: 27, Val: "LEFT"},
					{Typ: token.DESCRIPTION, Line: 3, Pos: 33, Val: `"up descr"`},
					{Typ: token.IDENT, Line: 3, Pos: 44, Val: "UP"},
					{Typ: token.DESCRIPTION, Line: 6, Pos: 48, Val: "\"\"\"\n\tright descr\n\t\"\"\""},
					{Typ: token.IDENT, Line: 7, Pos: 71, Val: "RIGHT"},
					{Typ: token.DESCRIPTION, Line: 8, Pos: 78, Val: `"down descr above"`},
					{Typ: token.DESCRIPTION, Line: 9, Pos: 98, Val: `"down descr before"`},
					{Typ: token.IDENT, Line: 9, Pos: 118, Val: "DOWN"},
					{Typ: token.RBRACE, Line: 10, Pos: 123, Val: "}"},
				}...)
				expectEOF(qt, l)
			})

			triT.Run("withDirectives", func(qt *testing.T) {
				fset := token.NewDocSet()
				src := []byte(`enum Rect {
	LEFT @green @blue
	UP @red
	RIGHT
	DOWN @red @green @blue
}`)
				l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
				expectItems(qt, l, []Item{
					{Typ: token.ENUM, Line: 1, Pos: 1, Val: "enum"},
					{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
					{Typ: token.LBRACE, Line: 1, Pos: 11, Val: "{"},
					{Typ: token.IDENT, Line: 2, Pos: 14, Val: "LEFT"},
					{Typ: token.AT, Line: 2, Pos: 19, Val: "@"},
					{Typ: token.IDENT, Line: 2, Pos: 20, Val: "green"},
					{Typ: token.AT, Line: 2, Pos: 26, Val: "@"},
					{Typ: token.IDENT, Line: 2, Pos: 27, Val: "blue"},
					{Typ: token.IDENT, Line: 3, Pos: 33, Val: "UP"},
					{Typ: token.AT, Line: 3, Pos: 36, Val: "@"},
					{Typ: token.IDENT, Line: 3, Pos: 37, Val: "red"},
					{Typ: token.IDENT, Line: 4, Pos: 42, Val: "RIGHT"},
					{Typ: token.IDENT, Line: 5, Pos: 49, Val: "DOWN"},
					{Typ: token.AT, Line: 5, Pos: 54, Val: "@"},
					{Typ: token.IDENT, Line: 5, Pos: 55, Val: "red"},
					{Typ: token.AT, Line: 5, Pos: 59, Val: "@"},
					{Typ: token.IDENT, Line: 5, Pos: 60, Val: "green"},
					{Typ: token.AT, Line: 5, Pos: 66, Val: "@"},
					{Typ: token.IDENT, Line: 5, Pos: 67, Val: "blue"},
					{Typ: token.RBRACE, Line: 6, Pos: 72, Val: "}"},
				}...)
				expectEOF(qt, l)
			})
		})

		subT.Run("AsInputFieldsDef", func(triT *testing.T) {

			triT.Run("simple", func(qt *testing.T) {
				fset := token.NewDocSet()
				src := []byte(`input Rect {
	one: One
	two: Two
}`)
				l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
				expectItems(qt, l, []Item{
					{Typ: token.INPUT, Line: 1, Pos: 1, Val: "input"},
					{Typ: token.IDENT, Line: 1, Pos: 7, Val: "Rect"},
					{Typ: token.LBRACE, Line: 1, Pos: 12, Val: "{"},
					{Typ: token.IDENT, Line: 2, Pos: 15, Val: "one"},
					{Typ: token.COLON, Line: 2, Pos: 18, Val: ":"},
					{Typ: token.IDENT, Line: 2, Pos: 20, Val: "One"},
					{Typ: token.IDENT, Line: 3, Pos: 25, Val: "two"},
					{Typ: token.COLON, Line: 3, Pos: 28, Val: ":"},
					{Typ: token.IDENT, Line: 3, Pos: 30, Val: "Two"},
					{Typ: token.RBRACE, Line: 4, Pos: 34, Val: "}"},
				}...)
				expectEOF(qt, l)
			})

			triT.Run("withDescrs", func(qt *testing.T) {
				fset := token.NewDocSet()
				src := []byte(`input Rect {
	"one descr" one: One
	"""
	two descr
	"""
	two: Two
}`)
				l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
				expectItems(qt, l, []Item{
					{Typ: token.INPUT, Line: 1, Pos: 1, Val: "input"},
					{Typ: token.IDENT, Line: 1, Pos: 7, Val: "Rect"},
					{Typ: token.LBRACE, Line: 1, Pos: 12, Val: "{"},
					{Typ: token.DESCRIPTION, Line: 2, Pos: 15, Val: `"one descr"`},
					{Typ: token.IDENT, Line: 2, Pos: 27, Val: "one"},
					{Typ: token.COLON, Line: 2, Pos: 30, Val: ":"},
					{Typ: token.IDENT, Line: 2, Pos: 32, Val: "One"},
					{Typ: token.DESCRIPTION, Line: 5, Pos: 37, Val: "\"\"\"\n\ttwo descr\n\t\"\"\""},
					{Typ: token.IDENT, Line: 6, Pos: 58, Val: "two"},
					{Typ: token.COLON, Line: 6, Pos: 61, Val: ":"},
					{Typ: token.IDENT, Line: 6, Pos: 63, Val: "Two"},
					{Typ: token.RBRACE, Line: 7, Pos: 67, Val: "}"},
				}...)
				expectEOF(qt, l)
			})

			triT.Run("withDefVal", func(qt *testing.T) {
				fset := token.NewDocSet()
				src := []byte(`input Rect {
	one: One = 123
	two: Two = "abc"
}`)
				l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
				expectItems(qt, l, []Item{
					{Typ: token.INPUT, Line: 1, Pos: 1, Val: "input"},
					{Typ: token.IDENT, Line: 1, Pos: 7, Val: "Rect"},
					{Typ: token.LBRACE, Line: 1, Pos: 12, Val: "{"},
					{Typ: token.IDENT, Line: 2, Pos: 15, Val: "one"},
					{Typ: token.COLON, Line: 2, Pos: 18, Val: ":"},
					{Typ: token.IDENT, Line: 2, Pos: 20, Val: "One"},
					{Typ: token.ASSIGN, Line: 2, Pos: 24, Val: "="},
					{Typ: token.INT, Line: 2, Pos: 26, Val: "123"},
					{Typ: token.IDENT, Line: 3, Pos: 31, Val: "two"},
					{Typ: token.COLON, Line: 3, Pos: 34, Val: ":"},
					{Typ: token.IDENT, Line: 3, Pos: 36, Val: "Two"},
					{Typ: token.ASSIGN, Line: 3, Pos: 40, Val: "="},
					{Typ: token.STRING, Line: 3, Pos: 42, Val: `"abc"`},
					{Typ: token.RBRACE, Line: 4, Pos: 48, Val: "}"},
				}...)
				expectEOF(qt, l)
			})

			triT.Run("withDirectives", func(qt *testing.T) {
				fset := token.NewDocSet()
				src := []byte(`input Rect {
	one: One @green @blue
	two: Two @blue
}`)
				l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
				expectItems(qt, l, []Item{
					{Typ: token.INPUT, Line: 1, Pos: 1, Val: "input"},
					{Typ: token.IDENT, Line: 1, Pos: 7, Val: "Rect"},
					{Typ: token.LBRACE, Line: 1, Pos: 12, Val: "{"},
					{Typ: token.IDENT, Line: 2, Pos: 15, Val: "one"},
					{Typ: token.COLON, Line: 2, Pos: 18, Val: ":"},
					{Typ: token.IDENT, Line: 2, Pos: 20, Val: "One"},
					{Typ: token.AT, Line: 2, Pos: 24, Val: "@"},
					{Typ: token.IDENT, Line: 2, Pos: 25, Val: "green"},
					{Typ: token.AT, Line: 2, Pos: 31, Val: "@"},
					{Typ: token.IDENT, Line: 2, Pos: 32, Val: "blue"},
					{Typ: token.IDENT, Line: 3, Pos: 38, Val: "two"},
					{Typ: token.COLON, Line: 3, Pos: 41, Val: ":"},
					{Typ: token.IDENT, Line: 3, Pos: 43, Val: "Two"},
					{Typ: token.AT, Line: 3, Pos: 47, Val: "@"},
					{Typ: token.IDENT, Line: 3, Pos: 48, Val: "blue"},
					{Typ: token.RBRACE, Line: 4, Pos: 53, Val: "}"},
				}...)
				expectEOF(qt, l)
			})
		})
	})

	t.Run("All", func(subT *testing.T) {
		// Note: This test does not use a valid GraphQL type decl.
		// 		 Instead, it uses a construction that is valid by the lexer and tests
		//		 the full capabilities of the lexObject stateFn.

		fset := token.NewDocSet()
		src := []byte(`type Rect implements Shape & Obj @green @blue {
	"one descr" one(): One @one @two

	"""
	two descr
	"""
	two("one descr" one: One = 1): Two! @one @two

	"three descr"
	thr(one: One = 1, two: Two): [Thr]! @one @two

	for(one: One = 1 @one @two, two: Two = 2 @one @two, thr: Thr = 3 @one @two): [For!]! @one @two
}`)
		l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
		expectItems(subT, l, []Item{
			{Typ: token.TYPE, Line: 1, Pos: 1, Val: "type"},
			{Typ: token.IDENT, Line: 1, Pos: 6, Val: "Rect"},
			{Typ: token.IMPLEMENTS, Line: 1, Pos: 11, Val: "implements"},
			{Typ: token.IDENT, Line: 1, Pos: 22, Val: "Shape"},
			{Typ: token.IDENT, Line: 1, Pos: 30, Val: "Obj"},
			{Typ: token.AT, Line: 1, Pos: 34, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 35, Val: "green"},
			{Typ: token.AT, Line: 1, Pos: 41, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 42, Val: "blue"},
			{Typ: token.LBRACE, Line: 1, Pos: 47, Val: "{"},
			{Typ: token.DESCRIPTION, Line: 2, Pos: 50, Val: `"one descr"`},
			{Typ: token.IDENT, Line: 2, Pos: 62, Val: "one"},
			{Typ: token.LPAREN, Line: 2, Pos: 65, Val: "("},
			{Typ: token.RPAREN, Line: 2, Pos: 66, Val: ")"},
			{Typ: token.COLON, Line: 2, Pos: 67, Val: ":"},
			{Typ: token.IDENT, Line: 2, Pos: 69, Val: "One"},
			{Typ: token.AT, Line: 2, Pos: 73, Val: "@"},
			{Typ: token.IDENT, Line: 2, Pos: 74, Val: "one"},
			{Typ: token.AT, Line: 2, Pos: 78, Val: "@"},
			{Typ: token.IDENT, Line: 2, Pos: 79, Val: "two"},
			{Typ: token.DESCRIPTION, Line: 6, Pos: 85, Val: "\"\"\"\n\ttwo descr\n\t\"\"\""},
			{Typ: token.IDENT, Line: 7, Pos: 106, Val: "two"},
			{Typ: token.LPAREN, Line: 7, Pos: 109, Val: "("},
			{Typ: token.DESCRIPTION, Line: 7, Pos: 110, Val: `"one descr"`},
			{Typ: token.IDENT, Line: 7, Pos: 122, Val: "one"},
			{Typ: token.COLON, Line: 7, Pos: 125, Val: ":"},
			{Typ: token.IDENT, Line: 7, Pos: 127, Val: "One"},
			{Typ: token.ASSIGN, Line: 7, Pos: 131, Val: "="},
			{Typ: token.INT, Line: 7, Pos: 133, Val: "1"},
			{Typ: token.RPAREN, Line: 7, Pos: 134, Val: ")"},
			{Typ: token.COLON, Line: 7, Pos: 135, Val: ":"},
			{Typ: token.IDENT, Line: 7, Pos: 137, Val: "Two"},
			{Typ: token.NOT, Line: 7, Pos: 140, Val: "!"},
			{Typ: token.AT, Line: 7, Pos: 142, Val: "@"},
			{Typ: token.IDENT, Line: 7, Pos: 143, Val: "one"},
			{Typ: token.AT, Line: 7, Pos: 147, Val: "@"},
			{Typ: token.IDENT, Line: 7, Pos: 148, Val: "two"},
			{Typ: token.DESCRIPTION, Line: 9, Pos: 154, Val: "\"three descr\""},
			{Typ: token.IDENT, Line: 10, Pos: 169, Val: "thr"},
			{Typ: token.LPAREN, Line: 10, Pos: 172, Val: "("},
			{Typ: token.IDENT, Line: 10, Pos: 173, Val: "one"},
			{Typ: token.COLON, Line: 10, Pos: 176, Val: ":"},
			{Typ: token.IDENT, Line: 10, Pos: 178, Val: "One"},
			{Typ: token.ASSIGN, Line: 10, Pos: 182, Val: "="},
			{Typ: token.INT, Line: 10, Pos: 184, Val: "1"},
			{Typ: token.IDENT, Line: 10, Pos: 187, Val: "two"},
			{Typ: token.COLON, Line: 10, Pos: 190, Val: ":"},
			{Typ: token.IDENT, Line: 10, Pos: 192, Val: "Two"},
			{Typ: token.RPAREN, Line: 10, Pos: 195, Val: ")"},
			{Typ: token.COLON, Line: 10, Pos: 196, Val: ":"},
			{Typ: token.LBRACK, Line: 10, Pos: 198, Val: "["},
			{Typ: token.IDENT, Line: 10, Pos: 199, Val: "Thr"},
			{Typ: token.RBRACK, Line: 10, Pos: 202, Val: "]"},
			{Typ: token.NOT, Line: 10, Pos: 203, Val: "!"},
			{Typ: token.AT, Line: 10, Pos: 205, Val: "@"},
			{Typ: token.IDENT, Line: 10, Pos: 206, Val: "one"},
			{Typ: token.AT, Line: 10, Pos: 210, Val: "@"},
			{Typ: token.IDENT, Line: 10, Pos: 211, Val: "two"},
			{Typ: token.IDENT, Line: 12, Pos: 217, Val: "for"},
			{Typ: token.LPAREN, Line: 12, Pos: 220, Val: "("},
			{Typ: token.IDENT, Line: 12, Pos: 221, Val: "one"},
			{Typ: token.COLON, Line: 12, Pos: 224, Val: ":"},
			{Typ: token.IDENT, Line: 12, Pos: 226, Val: "One"},
			{Typ: token.ASSIGN, Line: 12, Pos: 230, Val: "="},
			{Typ: token.INT, Line: 12, Pos: 232, Val: "1"},
			{Typ: token.AT, Line: 12, Pos: 234, Val: "@"},
			{Typ: token.IDENT, Line: 12, Pos: 235, Val: "one"},
			{Typ: token.AT, Line: 12, Pos: 239, Val: "@"},
			{Typ: token.IDENT, Line: 12, Pos: 240, Val: "two"},
			{Typ: token.IDENT, Line: 12, Pos: 245, Val: "two"},
			{Typ: token.COLON, Line: 12, Pos: 248, Val: ":"},
			{Typ: token.IDENT, Line: 12, Pos: 250, Val: "Two"},
			{Typ: token.ASSIGN, Line: 12, Pos: 254, Val: "="},
			{Typ: token.INT, Line: 12, Pos: 256, Val: "2"},
			{Typ: token.AT, Line: 12, Pos: 258, Val: "@"},
			{Typ: token.IDENT, Line: 12, Pos: 259, Val: "one"},
			{Typ: token.AT, Line: 12, Pos: 263, Val: "@"},
			{Typ: token.IDENT, Line: 12, Pos: 264, Val: "two"},
			{Typ: token.IDENT, Line: 12, Pos: 269, Val: "thr"},
			{Typ: token.COLON, Line: 12, Pos: 272, Val: ":"},
			{Typ: token.IDENT, Line: 12, Pos: 274, Val: "Thr"},
			{Typ: token.ASSIGN, Line: 12, Pos: 278, Val: "="},
			{Typ: token.INT, Line: 12, Pos: 280, Val: "3"},
			{Typ: token.AT, Line: 12, Pos: 282, Val: "@"},
			{Typ: token.IDENT, Line: 12, Pos: 283, Val: "one"},
			{Typ: token.AT, Line: 12, Pos: 287, Val: "@"},
			{Typ: token.IDENT, Line: 12, Pos: 288, Val: "two"},
			{Typ: token.RPAREN, Line: 12, Pos: 291, Val: ")"},
			{Typ: token.COLON, Line: 12, Pos: 292, Val: ":"},
			{Typ: token.LBRACK, Line: 12, Pos: 294, Val: "["},
			{Typ: token.IDENT, Line: 12, Pos: 295, Val: "For"},
			{Typ: token.NOT, Line: 12, Pos: 298, Val: "!"},
			{Typ: token.RBRACK, Line: 12, Pos: 299, Val: "]"},
			{Typ: token.NOT, Line: 12, Pos: 300, Val: "!"},
			{Typ: token.AT, Line: 12, Pos: 302, Val: "@"},
			{Typ: token.IDENT, Line: 12, Pos: 303, Val: "one"},
			{Typ: token.AT, Line: 12, Pos: 307, Val: "@"},
			{Typ: token.IDENT, Line: 12, Pos: 308, Val: "two"},
			{Typ: token.RBRACE, Line: 13, Pos: 312, Val: "}"},
		}...)
		expectEOF(subT, l)
	})
}

func TestLexUnion(t *testing.T) {

	t.Run("Simple", func(subT *testing.T) {
		fset := token.NewDocSet()
		src := []byte(`union Pizza = Triangle | Circle`)
		l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
		expectItems(subT, l, []Item{
			{Typ: token.UNION, Line: 1, Pos: 1, Val: "union"},
			{Typ: token.IDENT, Line: 1, Pos: 7, Val: "Pizza"},
			{Typ: token.ASSIGN, Line: 1, Pos: 13, Val: "="},
			{Typ: token.IDENT, Line: 1, Pos: 15, Val: "Triangle"},
			{Typ: token.IDENT, Line: 1, Pos: 26, Val: "Circle"},
		}...)
		expectEOF(subT, l)
	})

	t.Run("WithDirectives", func(subT *testing.T) {
		fset := token.NewDocSet()
		src := []byte(`union Pizza @ham @pineapple = Triangle | Circle`)
		l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
		expectItems(subT, l, []Item{
			{Typ: token.UNION, Line: 1, Pos: 1, Val: "union"},
			{Typ: token.IDENT, Line: 1, Pos: 7, Val: "Pizza"},
			{Typ: token.AT, Line: 1, Pos: 13, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 14, Val: "ham"},
			{Typ: token.AT, Line: 1, Pos: 18, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 19, Val: "pineapple"},
			{Typ: token.ASSIGN, Line: 1, Pos: 29, Val: "="},
			{Typ: token.IDENT, Line: 1, Pos: 31, Val: "Triangle"},
			{Typ: token.IDENT, Line: 1, Pos: 42, Val: "Circle"},
		}...)
		expectEOF(subT, l)
	})

	t.Run("Extension", func(subT *testing.T) {
		subT.Run("WithNothing", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`extend union Pizza`)
			l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
			expectItems(triT, l, []Item{
				{Typ: token.EXTEND, Line: 1, Pos: 1, Val: "extend"},
				{Typ: token.UNION, Line: 1, Pos: 8, Val: "union"},
				{Typ: token.IDENT, Line: 1, Pos: 14, Val: "Pizza"},
			}...)
			expectEOF(triT, l)
		})

		subT.Run("WithDirectives", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`extend union Pizza @ham @pineapple`)
			l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
			expectItems(triT, l, []Item{
				{Typ: token.EXTEND, Line: 1, Pos: 1, Val: "extend"},
				{Typ: token.UNION, Line: 1, Pos: 8, Val: "union"},
				{Typ: token.IDENT, Line: 1, Pos: 14, Val: "Pizza"},
				{Typ: token.AT, Line: 1, Pos: 20, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 21, Val: "ham"},
				{Typ: token.AT, Line: 1, Pos: 25, Val: "@"},
				{Typ: token.IDENT, Line: 1, Pos: 26, Val: "pineapple"},
			}...)
			expectEOF(triT, l)
		})

		subT.Run("WithMembers", func(triT *testing.T) {
			fset := token.NewDocSet()
			src := []byte(`extend union Pizza = Triangle | Circle`)
			l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)
			expectItems(triT, l, []Item{
				{Typ: token.EXTEND, Line: 1, Pos: 1, Val: "extend"},
				{Typ: token.UNION, Line: 1, Pos: 8, Val: "union"},
				{Typ: token.IDENT, Line: 1, Pos: 14, Val: "Pizza"},
				{Typ: token.ASSIGN, Line: 1, Pos: 20, Val: "="},
				{Typ: token.IDENT, Line: 1, Pos: 22, Val: "Triangle"},
				{Typ: token.IDENT, Line: 1, Pos: 33, Val: "Circle"},
			}...)
			expectEOF(triT, l)
		})
	})
}

func TestLexDirective(t *testing.T) {

	t.Run("Simple", func(subT *testing.T) {
		fset := token.NewDocSet()
		src := []byte(`directive @skip on FIELD | FIELD_DEFINITION`)
		l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)

		expectItems(subT, l, []Item{
			{Typ: token.DIRECTIVE, Line: 1, Pos: 1, Val: "directive"},
			{Typ: token.AT, Line: 1, Pos: 11, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 12, Val: "skip"},
			{Typ: token.ON, Line: 1, Pos: 17, Val: "on"},
			{Typ: token.IDENT, Line: 1, Pos: 20, Val: "FIELD"},
			{Typ: token.IDENT, Line: 1, Pos: 28, Val: "FIELD_DEFINITION"},
		}...)
		expectEOF(subT, l)
	})

	t.Run("WithArgs", func(subT *testing.T) {
		fset := token.NewDocSet()
		src := []byte(`directive @skip(if: Boolean, else: Boolean = false) on FIELD | FIELD_DEFINITION`)
		l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)

		expectItems(subT, l, []Item{
			{Typ: token.DIRECTIVE, Line: 1, Pos: 1, Val: "directive"},
			{Typ: token.AT, Line: 1, Pos: 11, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 12, Val: "skip"},
			{Typ: token.LPAREN, Line: 1, Pos: 16, Val: "("},
			{Typ: token.IDENT, Line: 1, Pos: 17, Val: "if"},
			{Typ: token.COLON, Line: 1, Pos: 19, Val: ":"},
			{Typ: token.IDENT, Line: 1, Pos: 21, Val: "Boolean"},
			{Typ: token.IDENT, Line: 1, Pos: 30, Val: "else"},
			{Typ: token.COLON, Line: 1, Pos: 34, Val: ":"},
			{Typ: token.IDENT, Line: 1, Pos: 36, Val: "Boolean"},
			{Typ: token.ASSIGN, Line: 1, Pos: 44, Val: "="},
			{Typ: token.IDENT, Line: 1, Pos: 46, Val: "false"},
			{Typ: token.RPAREN, Line: 1, Pos: 51, Val: ")"},
			{Typ: token.ON, Line: 1, Pos: 53, Val: "on"},
			{Typ: token.IDENT, Line: 1, Pos: 56, Val: "FIELD"},
			{Typ: token.IDENT, Line: 1, Pos: 64, Val: "FIELD_DEFINITION"},
		}...)
		expectEOF(subT, l)
	})

	t.Run("ArgsWithDirectives", func(subT *testing.T) {
		fset := token.NewDocSet()
		src := []byte(`directive @skip(if: Boolean @one(), else: Boolean = false @one() @two()) on FIELD | FIELD_DEFINITION`)
		l := Lex(fset.AddDoc("", fset.Base(), len(src)), src, 0)

		expectItems(subT, l, []Item{
			{Typ: token.DIRECTIVE, Line: 1, Pos: 1, Val: "directive"},
			{Typ: token.AT, Line: 1, Pos: 11, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 12, Val: "skip"},
			{Typ: token.LPAREN, Line: 1, Pos: 16, Val: "("},
			{Typ: token.IDENT, Line: 1, Pos: 17, Val: "if"},
			{Typ: token.COLON, Line: 1, Pos: 19, Val: ":"},
			{Typ: token.IDENT, Line: 1, Pos: 21, Val: "Boolean"},
			{Typ: token.AT, Line: 1, Pos: 29, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 30, Val: "one"},
			{Typ: token.LPAREN, Line: 1, Pos: 33, Val: "("},
			{Typ: token.RPAREN, Line: 1, Pos: 34, Val: ")"},
			{Typ: token.IDENT, Line: 1, Pos: 37, Val: "else"},
			{Typ: token.COLON, Line: 1, Pos: 41, Val: ":"},
			{Typ: token.IDENT, Line: 1, Pos: 43, Val: "Boolean"},
			{Typ: token.ASSIGN, Line: 1, Pos: 51, Val: "="},
			{Typ: token.IDENT, Line: 1, Pos: 53, Val: "false"},
			{Typ: token.AT, Line: 1, Pos: 59, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 60, Val: "one"},
			{Typ: token.LPAREN, Line: 1, Pos: 63, Val: "("},
			{Typ: token.RPAREN, Line: 1, Pos: 64, Val: ")"},
			{Typ: token.AT, Line: 1, Pos: 66, Val: "@"},
			{Typ: token.IDENT, Line: 1, Pos: 67, Val: "two"},
			{Typ: token.LPAREN, Line: 1, Pos: 70, Val: "("},
			{Typ: token.RPAREN, Line: 1, Pos: 71, Val: ")"},
			{Typ: token.RPAREN, Line: 1, Pos: 72, Val: ")"},
			{Typ: token.ON, Line: 1, Pos: 74, Val: "on"},
			{Typ: token.IDENT, Line: 1, Pos: 77, Val: "FIELD"},
			{Typ: token.IDENT, Line: 1, Pos: 85, Val: "FIELD_DEFINITION"},
		}...)
		expectEOF(subT, l)
	})
}

func expectItems(t *testing.T, l Interface, items ...Item) {
	for _, item := range items {
		lItem := l.NextItem()
		if lItem != item {
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

func TestLex(t *testing.T) {
	dset := token.NewDocSet()
	l := Lex(dset.AddDoc("", dset.Base(), len(gqlSrc)), gqlSrc, 0)

	expectItems(t, l, testItems.Items...)
	expectEOF(t, l)
}

func BenchmarkLex(b *testing.B) {
	d := token.NewDocSet().AddDoc("test", -1, len(gqlSrc))
	for i := 0; i < b.N; i++ {
		l := Lex(d, gqlSrc, 0)
		l.Drain()
	}
}

func TestLex2(t *testing.T) {
	dset := token.NewDocSet()
	l := Lex2(dset.AddDoc("", dset.Base(), len(gqlSrc)), string(gqlSrc))

	expectItems(t, l, testItems.Items...)
	expectEOF(t, l)
}

func BenchmarkLex2(b *testing.B) {
	b.Run("ToStringBeforeAll", func(subB *testing.B) {
		benchSrcStr := string(gqlSrc)

		d := token.NewDocSet().AddDoc("test", -1, len(benchSrcStr))
		for i := 0; i < subB.N; i++ {
			l := Lex2(d, benchSrcStr)
			l.Drain()
		}
	})

	b.Run("ToStringBeforeEach", func(subB *testing.B) {
		d := token.NewDocSet().AddDoc("test", -1, len(gqlSrc))
		for i := 0; i < subB.N; i++ {
			benchSrcStr := string(gqlSrc)
			l := Lex2(d, benchSrcStr)
			l.Drain()
		}
	})
}
