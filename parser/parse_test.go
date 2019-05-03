package parser

import (
	"fmt"
	"github.com/gqlc/graphql/ast"
	"github.com/gqlc/graphql/token"
	"strings"
	"testing"
)

func TestParseDoc(t *testing.T) {

	t.Run("topLvlDirectives", func(subT *testing.T) {
		topLvlDirectives := `@test(one: 1, two: 2, thr: 3)`
		doc, err := parse("topLvlDirectives", topLvlDirectives)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Directives) != 1 {
			subT.Fail()
			return
		}
	})

	t.Run("schema", func(subT *testing.T) {
		subT.Run("perfect", func(triT *testing.T) {
			schema := `schema @one @two() @three(a: "A") {
	query: Query
	mutation: Mutation
	subscription: Subscription
}`
			doc, err := parse("perfect", schema)
			if err != nil {
				triT.Error(err)
				return
			}

			if doc.Schema == nil {
				triT.Fail()
				return
			}

			spec := doc.Schema.Spec.(*ast.TypeDecl_TypeSpec).TypeSpec
			if len(spec.Directives) != 3 {
				triT.Fail()
				return
			}

			s := spec.Type.(*ast.TypeSpec_Schema).Schema
			if len(s.RootOps.List) != 3 {
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

		spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeSpec).TypeSpec
		if len(spec.Directives) == 0 {
			subT.Fail()
			return
		}
	})

	t.Run("object", func(subT *testing.T) {
		obj := `type Test implements One & Two & Thr @one @two {
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

		spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeSpec).TypeSpec
		if len(spec.Directives) == 0 {
			subT.Fail()
			return
		}

		o := spec.Type.(*ast.TypeSpec_Object).Object
		fmt.Println(o)
		if len(o.Fields.List) != 4 {
			subT.Fail()
			return
		}

		if len(o.Interfaces) != 3 {
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

		spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeSpec).TypeSpec
		if len(spec.Directives) == 0 {
			subT.Fail()
			return
		}

		o := spec.Type.(*ast.TypeSpec_Interface).Interface
		if len(o.Fields.List) != 4 {
			subT.Fail()
		}
	})

	t.Run("union", func(subT *testing.T) {
		uni := `union Test @one @two = One | Two | Three | Four`
		doc, err := parse("union", uni)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Types) == 0 {
			subT.Fail()
			return
		}

		spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeSpec).TypeSpec
		if len(spec.Directives) == 0 {
			subT.Fail()
			return
		}

		o := spec.Type.(*ast.TypeSpec_Union).Union
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

		spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeSpec).TypeSpec
		if len(spec.Directives) != 2 {
			subT.Fail()
			return
		}

		o := spec.Type.(*ast.TypeSpec_Enum).Enum
		if len(o.Values.List) != 3 {
			subT.Fail()
		}
	})

	t.Run("input", func(subT *testing.T) {
		inp := `input Test @one @two {
				one: One @one
				two: Two = 2 @one @two
				three: Three @one @two @three
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

		spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeSpec).TypeSpec
		if len(spec.Directives) != 2 {
			subT.Fail()
			return
		}

		o := spec.Type.(*ast.TypeSpec_Input).Input
		if len(o.Fields.List) != 3 {
			subT.Fail()
			return
		}

		iType := o.Fields.List[2]
		if len(iType.Directives) != 3 {
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

		spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeSpec).TypeSpec

		o := spec.Type.(*ast.TypeSpec_Directive).Directive
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

		spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeExtSpec).TypeExtSpec

		o := spec.Type
		if o.Type == nil {
			subT.Fail()
			return
		}

		if len(o.Directives) != 1 {
			subT.Fail()
			return
		}
	})
}

func parse(name, src string) (*ast.Document, error) {
	return ParseDoc(token.NewDocSet(), name, strings.NewReader(src), 0)
}
