package parser

import (
	"github.com/gqlc/graphql/ast"
	"github.com/gqlc/graphql/token"
	"strings"
	"testing"
)

func TestParseDoc(t *testing.T) {
	t.Run("imports", func(subT *testing.T) {
		justImports := `import (
	"one.gql"
	"two.gql"
	"three"
)`
		doc, err := parse("justImports", justImports)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Imports) == 0 {
			subT.Fail()
			return
		}

		if len(doc.Imports[0].Specs) != 3 {
			subT.Fail()
			return
		}

		for _, is := range doc.Imports[0].Specs {
			if is.Name.Name != "one" && is.Name.Name != "two" && is.Name.Name != "three" {
				subT.Fail()
				return
			}
		}

		subT.Run("single", func(triT *testing.T) {
			singleImport := `import "one"`

			doc, err := parse("single", singleImport)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Imports) == 0 {
				triT.Fail()
				return
			}

			if len(doc.Imports[0].Specs) != 1 {
				triT.Fail()
				return
			}
		})

		subT.Run("singleWithTypeDecl", func(triT *testing.T) {
			singleImport := `import "one"

type Test {}`

			doc, err := parse("singleWithTypeDecl", singleImport)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Imports) == 0 {
				triT.Fail()
				return
			}

			if len(doc.Imports[0].Specs) != 1 {
				triT.Fail()
				return
			}

			if len(doc.Types) != 1 {
				triT.Fail()
				return
			}

			if doc.Types[0].Spec.(*ast.TypeSpec).Name.(*ast.Ident).Name != "Test" {
				triT.Fail()
			}
		})
	})

	t.Run("schema", func(subT *testing.T) {
		subT.Run("perfect", func(triT *testing.T) {
			schema := `schema @one @two() @three(a: "A") {
	query: Query!
	mutation: Mutation
	subscription: Subscription
}`
			doc, err := parse("perfect", schema)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Schemas) == 0 {
				triT.Fail()
				return
			}

			spec := doc.Schemas[0].Spec.(*ast.TypeSpec)
			if len(spec.Dirs) != 3 {
				triT.Fail()
				return
			}

			s := spec.Type.(*ast.SchemaType)
			if len(s.Fields.List) != 3 {
				triT.Fail()
				return
			}
		})

		subT.Run("invalid", func(triT *testing.T) {
			schema := `schema {
	query: Query
	mut: Mutation
}`
			_, err := parse("invalid", schema)
			if err == nil {
				triT.Fail()
				return
			}
		})
	})

	t.Run("scalar", func(subT *testing.T) {
		scalar := `scalar Test @one @two() @three(a: 1, b: 2)`
		doc, err := parse("scalar", scalar)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Types) == 0 {
			subT.Fail()
			return
		}

		spec := doc.Types[0].Spec.(*ast.TypeSpec)
		if len(spec.Dirs) == 0 {
			subT.Fail()
			return
		}
	})

	t.Run("object", func(subT *testing.T) {
		obj := `type Test implements One & Two & test.Thr @one @two {
				one(): One @one @two
				two(one: One): Two! @one @two
				thr(one: One = 1, two: Two): [Thr]! @one @two
				for(one: One = 1 @one @two, two: Two = 2 @one @two, thr: Thr = 3 @one @two): [For!]! @one @two 
			}`
		doc, err := parse("object", obj)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Types) == 0 {
			subT.Fail()
			return
		}

		spec := doc.Types[0].Spec.(*ast.TypeSpec)
		if len(spec.Dirs) == 0 {
			subT.Fail()
			return
		}

		o := spec.Type.(*ast.ObjectType)
		if len(o.Fields.List) != 4 {
			subT.Fail()
			return
		}

		if len(o.Impls) != 3 {
			subT.Fail()
		}
	})

	t.Run("interface", func(subT *testing.T) {
		inter := `interface One @one @two {
				one(): One @one @two
				two(one: One): Two! @one @two
				thr(one: One = 1, two: Two): [Thr]! @one @two
				for(one: One = 1 @one @two, two: Two = 2 @one @two, thr: Thr = 3 @one @two): [For!]! @one @two
			}`
		doc, err := parse("interface", inter)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Types) == 0 {
			subT.Fail()
			return
		}

		spec := doc.Types[0].Spec.(*ast.TypeSpec)
		if len(spec.Dirs) == 0 {
			subT.Fail()
			return
		}

		o := spec.Type.(*ast.InterfaceType)
		if len(o.Fields.List) != 4 {
			subT.Fail()
		}
	})

	t.Run("union", func(subT *testing.T) {
		uni := `union Test @one @two = One | Two | Three | test.Four`
		doc, err := parse("union", uni)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Types) == 0 {
			subT.Fail()
			return
		}

		spec := doc.Types[0].Spec.(*ast.TypeSpec)
		if len(spec.Dirs) == 0 {
			subT.Fail()
			return
		}

		o := spec.Type.(*ast.UnionType)
		if len(o.Members) != 4 {
			subT.Fail()
		}
	})

	t.Run("enum", func(subT *testing.T) {
		enu := `enum Test @one @two {
				"One before" ONE @one
				"""
				Two above
				"""
				TWO	@one @two
				"Three above"
				"Three before" THREE @one @two @three
			}`
		doc, err := parse("enum", enu)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Types) == 0 {
			subT.Fail()
			return
		}

		spec := doc.Types[0].Spec.(*ast.TypeSpec)
		if len(spec.Dirs) != 2 {
			subT.Fail()
			return
		}

		o := spec.Type.(*ast.EnumType)
		if len(o.Fields.List) != 3 {
			subT.Fail()
		}
	})

	t.Run("input", func(subT *testing.T) {
		inp := `input Test @one @two {
				one: One @one
				two: Two = 2 @one @two
				three: test.Three @one @two @three
			}`
		doc, err := parse("input", inp)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Types) == 0 {
			subT.Fail()
			return
		}

		spec := doc.Types[0].Spec.(*ast.TypeSpec)
		if len(spec.Dirs) != 2 {
			subT.Fail()
			return
		}

		o := spec.Type.(*ast.InputType)
		if len(o.Fields.List) != 3 {
			subT.Fail()
			return
		}

		iType := o.Fields.List[2]
		if len(iType.Dirs) != 3 {
			subT.Fail()
			return
		}

		if _, ok := iType.Type.(*ast.SelectorExpr); !ok {
			subT.Fail()
			return
		}
	})

	t.Run("directive", func(subT *testing.T) {
		dir := `directive @test(one: One = 1 @one, two: Two = 2 @one @two) on SCHEMA | FIELD_DEFINITION`
		doc, err := parse("directive", dir)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Types) == 0 {
			subT.Fail()
			return
		}

		spec := doc.Types[0].Spec.(*ast.TypeSpec)

		o := spec.Type.(*ast.DirectiveType)
		if len(o.Args.List) != 2 {
			subT.Fail()
		}
		if len(o.Locs) != 2 {
			subT.Fail()
		}
	})

	t.Run("extension", func(subT *testing.T) {
		ex := `extend type Test @one`
		doc, err := parse("extension", ex)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Types) == 0 {
			subT.Fail()
			return
		}

		spec := doc.Types[0].Spec.(*ast.TypeExtensionSpec)

		o := spec.Type
		if o.Type == nil {
			subT.Fail()
		}
	})

	t.Run("importIdent", func(subT *testing.T) {
		ex := `extend type one.Test @one`
		doc, err := parse("extension", ex)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Types) == 0 {
			subT.Fail()
			return
		}

		spec := doc.Types[0].Spec.(*ast.TypeExtensionSpec)

		o := spec.Type
		if o.Name == nil {
			subT.Fail()
			return
		}

		name := o.Name.(*ast.SelectorExpr)
		if name.X.(*ast.Ident).Name != "one" || name.Sel.Name != "Test" {
			subT.Fail()
		}
	})
}

func parse(name, src string) (*ast.Document, error) {
	return ParseDoc(token.NewDocSet(), name, strings.NewReader(src), 0)
}
