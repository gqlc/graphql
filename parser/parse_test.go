package parser

import (
	"fmt"
	"github.com/gqlc/graphql/ast"
	"github.com/gqlc/graphql/lexer"
	"github.com/gqlc/graphql/token"
	"strings"
	"testing"
)

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
			"one":  {Kind: int64(token.INT), Value: "1"},
			"two":  {Kind: int64(token.STRING), Value: `"2"`},
			"thr":  {Kind: int64(token.FLOAT), Value: "3.0"},
			"four": {Kind: int64(token.IDENT), Value: "true"},
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

		bl, ok := l.ListLit.List.(*ast.ListLit_BasicList)
		if !ok {
			subT.Fail()
			return
		}

		vals := map[string]int{"1": 0, `"1"`: 0, "1.1": 0, "true": 0}
		for _, e := range bl.BasicList.Values {
			delete(vals, e.Value)
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

func TestParseDoc(t *testing.T) {

	t.Run("TopLvlDirectives", func(subT *testing.T) {
		topLvlDirectives := `@import(one: 1, two: 2, thr: 3)`
		doc, err := parse("TopLvlDirectives", topLvlDirectives)
		if err != nil {
			subT.Error(err)
			return
		}

		if len(doc.Directives) != 1 {
			subT.Fail()
			return
		}
	})

	t.Run("Schema", func(subT *testing.T) {
		subT.Run("Perfect", func(triT *testing.T) {
			schema := `schema @one @two() @three(a: "A") {
	query: Query
	mutation: Mutation
	subscription: Subscription
}`
			doc, err := parse("Perfect", schema)
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

		subT.Run("Invalid", func(triT *testing.T) {
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

		subT.Run("Extension", func(triT *testing.T) {
			schema := `extend schema @one @two() @three(a: "A") {
	query: Query
	mutation: Mutation
	subscription: Subscription
}`
			doc, err := parse("Extension", schema)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Types) != 1 {
				triT.Fail()
				return
			}

			spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeExtSpec).TypeExtSpec.Type
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
	})

	t.Run("Scalar", func(subT *testing.T) {
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

		subT.Run("Extension", func(triT *testing.T) {
			scalar := `extend scalar Test @one @two() @three(a: 1, b: 2)`
			doc, err := parse("scalar", scalar)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Types) != 1 {
				triT.Fail()
				return
			}

			spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeExtSpec).TypeExtSpec.Type
			if len(spec.Directives) == 0 {
				triT.Fail()
				return
			}
		})
	})

	t.Run("Object", func(subT *testing.T) {
		obj := `type Test implements One & Two & Thr @one @two {
				one(): One @one @two
				two(one: One): Two! @one @two
				thr(one: One = 1, two: Two): [Thr]! @one @two
				for(one: One = 1 @one @two, two: Two = 2 @one @two, thr: Thr = 3 @one @two): [For!]! @one @two 
			}`
		doc, err := parse("Object", obj)
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
		if len(o.Fields.List) != 4 {
			subT.Fail()
			return
		}

		if len(o.Interfaces) != 3 {
			subT.Fail()
		}

		subT.Run("Extension", func(triT *testing.T) {
			obj := `extend type Test implements One & Two & Thr @one @two {
				one(): One @one @two
				two(one: One): Two! @one @two
				thr(one: One = 1, two: Two): [Thr]! @one @two
				for(one: One = 1 @one @two, two: Two = 2 @one @two, thr: Thr = 3 @one @two): [For!]! @one @two 
			}`
			doc, err := parse("Object", obj)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Types) != 1 {
				triT.Fail()
				return
			}

			spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeExtSpec).TypeExtSpec.Type
			if len(spec.Directives) == 0 {
				triT.Fail()
				return
			}

			o := spec.Type.(*ast.TypeSpec_Object).Object
			if len(o.Fields.List) != 4 {
				triT.Fail()
				return
			}

			if len(o.Interfaces) != 3 {
				triT.Fail()
			}
		})
	})

	t.Run("Interface", func(subT *testing.T) {
		inter := `interface One @one @two {
				one(): One @one @two
				two(one: One): Two! @one @two
				thr(one: One = 1, two: Two): [Thr]! @one @two
				for(one: One = 1 @one @two, two: Two = 2 @one @two, thr: Thr = 3 @one @two): [For!]! @one @two
			}`
		doc, err := parse("Interface", inter)
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

		subT.Run("Extension", func(triT *testing.T) {
			inter := `extend interface One @one @two {
				one(): One @one @two
				two(one: One): Two! @one @two
				thr(one: One = 1, two: Two): [Thr]! @one @two
				for(one: One = 1 @one @two, two: Two = 2 @one @two, thr: Thr = 3 @one @two): [For!]! @one @two
			}`
			doc, err := parse("Interface", inter)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Types) != 1 {
				triT.Fail()
				return
			}

			spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeExtSpec).TypeExtSpec.Type
			if len(spec.Directives) == 0 {
				triT.Fail()
				return
			}

			o := spec.Type.(*ast.TypeSpec_Interface).Interface
			if len(o.Fields.List) != 4 {
				triT.Fail()
			}
		})
	})

	t.Run("Union", func(subT *testing.T) {
		subT.Run("NoMemberOrDirectives", func(triT *testing.T) {
			uni := `union Test`
			doc, err := parse("NoMembersOrDirectives", uni)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Types) == 0 {
				triT.Fail()
				return
			}

			spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeSpec).TypeSpec

			o := spec.Type.(*ast.TypeSpec_Union).Union
			if len(o.Members) != 0 {
				triT.Fail()
			}
		})

		subT.Run("SingleMember", func(triT *testing.T) {
			uni := `union Test = One`
			doc, err := parse("Union", uni)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Types) == 0 {
				triT.Fail()
				return
			}

			spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeSpec).TypeSpec
			o := spec.Type.(*ast.TypeSpec_Union).Union
			if len(o.Members) != 1 {
				triT.Fail()
			}
		})

		subT.Run("WithDirectivesAndMembers", func(triT *testing.T) {
			uni := `union Test @one @two = One | Two | Three | Four`
			doc, err := parse("Union", uni)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Types) == 0 {
				triT.Fail()
				return
			}

			spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeSpec).TypeSpec
			if len(spec.Directives) == 0 {
				triT.Fail()
				return
			}

			o := spec.Type.(*ast.TypeSpec_Union).Union
			if len(o.Members) != 4 {
				triT.Fail()
			}
		})

		subT.Run("Extension", func(triT *testing.T) {
			uni := `extend union Test @one @two = One | Two | Three | Four`
			doc, err := parse("Union", uni)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Types) != 1 {
				triT.Fail()
				return
			}

			spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeExtSpec).TypeExtSpec.Type
			if len(spec.Directives) == 0 {
				triT.Fail()
				return
			}

			o := spec.Type.(*ast.TypeSpec_Union).Union
			if len(o.Members) != 4 {
				triT.Fail()
			}
		})
	})

	t.Run("Enum", func(subT *testing.T) {
		enu := `enum Test @one @two {
				"One before" ONE @one
				"""
				Two above
				"""
				TWO	@one @two
				"Three above"
				"Three before" THREE @one @two @three
			}`
		doc, err := parse("Enum", enu)
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

		subT.Run("Extension", func(triT *testing.T) {
			enu := `extend enum Test @one @two {
				"One before" ONE @one
				"""
				Two above
				"""
				TWO	@one @two
				"Three above"
				"Three before" THREE @one @two @three
			}`
			doc, err := parse("Enum", enu)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Types) != 1 {
				triT.Fail()
				return
			}

			spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeExtSpec).TypeExtSpec.Type
			if len(spec.Directives) != 2 {
				triT.Fail()
				return
			}

			o := spec.Type.(*ast.TypeSpec_Enum).Enum
			if len(o.Values.List) != 3 {
				triT.Fail()
			}
		})
	})

	t.Run("Input", func(subT *testing.T) {
		inp := `input Test @one @two {
				one: One @one
				two: Two = 2 @one @two
				three: Three @one @two @three
			}`
		doc, err := parse("Input", inp)
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

		subT.Run("Extension", func(triT *testing.T) {
			inp := `extend input Test @one @two {
				one: One @one
				two: Two = 2 @one @two
				three: Three @one @two @three
			}`
			doc, err := parse("Input", inp)
			if err != nil {
				triT.Error(err)
				return
			}

			if len(doc.Types) != 1 {
				triT.Fail()
				return
			}

			spec := doc.Types[0].Spec.(*ast.TypeDecl_TypeExtSpec).TypeExtSpec.Type
			if len(spec.Directives) != 2 {
				triT.Fail()
				return
			}

			o := spec.Type.(*ast.TypeSpec_Input).Input
			if len(o.Fields.List) != 3 {
				triT.Fail()
				return
			}

			iType := o.Fields.List[2]
			if len(iType.Directives) != 3 {
				subT.Fail()
				return
			}
		})
	})

	t.Run("Directive", func(subT *testing.T) {
		dir := `directive @test(one: One = 1 @one, two: Two = 2 @one @two) on SCHEMA | FIELD_DEFINITION`
		doc, err := parse("Directive", dir)
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

		subT.Run("InvalidExtension", func(triT *testing.T) {
			dirExt := `extend directive @test`
			_, err := parse("DirectiveExt", dirExt)
			if err == nil {
				triT.Fail()
				return
			}

			if err.Error() != fmt.Sprintf("parser: %s: unexpected %s in parseExtension", "DirectiveExt:1", lexer.Item{Typ: token.DIRECTIVE, Val: "directive"}) {
				triT.Fail()
				return
			}
		})
	})
}

func parse(name, src string) (*ast.Document, error) {
	return ParseDoc(token.NewDocSet(), name, strings.NewReader(src), 0)
}
