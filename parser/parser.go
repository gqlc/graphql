package parser

import (
	"fmt"
	"runtime"

	"github.com/gqlc/graphql/ast"
	"github.com/gqlc/graphql/lexer"
	"github.com/gqlc/graphql/token"
)

type parser struct {
	doc  *token.Doc
	l    lexer.Interface
	name string
	line int
	pk   lexer.Item
	mode Mode

	schema *ast.TypeDecl

	dg, cdg []*ast.DocGroup_Doc
	direcs  []*ast.DirectiveLit
	dargs   []*ast.Arg
	fields  []*ast.Field

	args, fargs []*ast.InputValue
}

// next returns the next token
func (p *parser) next() (i lexer.Item) {
	defer func() {
		if i.Line > p.line {
			p.line = i.Line
		}
	}()

	if p.pk.Line != 0 {
		i = p.pk
		p.pk = lexer.Item{}
		return
	}
	return p.l.NextItem()
}

// peek peeks the next token
func (p *parser) peek() lexer.Item {
	p.pk = p.l.NextItem()
	return p.pk
}

func (p *parser) ignore() { p.pk = lexer.Item{} }

// expect consumes the next token and guarantees it has the required type.
func (p *parser) expect(tok token.Token, context string) lexer.Item {
	i := p.l.NextItem()
	if i.Typ != tok {
		p.unexpected(i, context)
	}
	return i
}

// errorf formats the error and terminates processing.
func (p *parser) errorf(format string, args ...interface{}) {
	format = fmt.Sprintf("parser: %s:%d: %s", p.name, p.line, format)
	panic(fmt.Errorf(format, args...))
}

// error terminates processing.
func (p *parser) error(err error) {
	p.errorf("%s", err)
}

// unexpected complains about the token and terminates processing.
func (p *parser) unexpected(token lexer.Item, context string) {
	p.errorf("unexpected %s in %s", token, context)
}

// recover is the handler that turns panics into returns from the top level of parse.
func (p *parser) recover(err *error) {
	e := recover()
	if e != nil {
		if _, ok := e.(runtime.Error); ok {
			panic(e)
		}
		if p != nil {
			p.l.Drain()
			p.l = nil
		}
		*err = e.(error)
	}
}

func (p *parser) parse(tokDoc *token.Doc, b []byte, mode Mode) (doc *ast.Document, err error) {
	defer p.recover(&err)
	p.l = lexer.Lex(tokDoc, string(b))
	p.doc = tokDoc
	p.mode = mode

	doc = &ast.Document{
		Name: p.name,
	}
	docs := p.parseDoc(&doc.Types, &doc.Directives)
	if len(docs) > 0 {
		doc.Doc = &ast.DocGroup{
			List: docs,
		}
	}

	if p.schema != nil {
		doc.Schema = p.schema
	}
	return
}

func (p *parser) parseDoc(types *[]*ast.TypeDecl, directives *[]*ast.DirectiveLit) (docs []*ast.DocGroup_Doc) {
	var cdocs []*ast.DocGroup_Doc
	ts := new(ast.TypeSpec)
	for {
		item := p.next()
		switch {
		case item.Typ == token.Token_EOF:
			return
		case item.Typ == token.Token_ERR:
			p.unexpected(item, "parseDoc")
		case item.Typ == token.Token_EXTEND:
			typ := p.next()
			if !typ.Typ.IsKeyword() {
				p.unexpected(typ, "parseDef:Extension")
			}

			ts.Reset()
			p.parseDef(typ, &cdocs, ts)

			extTs := *ts
			td := &ast.TypeDecl{
				TokPos: int64(item.Pos),
				Tok:    item.Typ,
				Spec: &ast.TypeDecl_TypeExtSpec{
					TypeExtSpec: &ast.TypeExtensionSpec{
						TokPos: int64(typ.Pos),
						Tok:    typ.Typ,
						Type:   &extTs,
					},
				},
			}
			if dLen := len(cdocs); dLen > 0 {
				td.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, dLen)}
				copy(td.Doc.List, cdocs)
				cdocs = cdocs[:0]
			}

			*types = append(*types, td)
		case item.Typ.IsKeyword():
			ts.Reset()

			p.parseDef(item, &cdocs, ts)

			tts := *ts
			td := &ast.TypeDecl{
				TokPos: int64(item.Pos),
				Tok:    item.Typ,
				Spec:   &ast.TypeDecl_TypeSpec{TypeSpec: &tts},
			}
			if dLen := len(cdocs); dLen > 0 {
				td.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, dLen)}
				copy(td.Doc.List, cdocs)
				cdocs = cdocs[:0]
			}

			*types = append(*types, td)

			if item.Typ == token.Token_SCHEMA {
				p.schema = td
			}
		case item.Typ == token.Token_COMMENT && p.mode&ParseComments != 0 || item.Typ == token.Token_DESCRIPTION:
			d := &ast.DocGroup_Doc{
				Text:    item.Val,
				Char:    int64(item.Pos),
				Comment: item.Typ == token.Token_COMMENT,
			}

			if len(cdocs) == 0 {
				cdocs = append(cdocs, d)
				break
			}

			prev := cdocs[len(cdocs)-1]
			lprev := p.doc.Line(token.Pos(int(prev.Char) + len(prev.Text)))
			if p.doc.Line(token.Pos(d.Char))-lprev == 1 {
				cdocs = append(cdocs, d)
				break
			}

			docs = append(docs, cdocs...)
			cdocs = cdocs[:0]
			cdocs = append(cdocs, d)
		case item.Typ == token.Token_AT:
			p.pk = item
			p.parseDirectives(directives)
		case item.Typ == token.Token_COMMENT:
		default:
			p.unexpected(item, "parseDoc:UnknownToken")
		}
	}
}

func (p *parser) parseDef(item lexer.Item, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
	switch item.Typ {
	case token.Token_TYPE:
		p.parseObject(item.Pos, item.Line, docs, ts)
	case token.Token_INPUT:
		p.parseInput(item.Pos, item.Line, docs, ts)
	case token.Token_INTERFACE:
		p.parseInterface(item.Pos, item.Line, docs, ts)
	case token.Token_UNION:
		p.parseUnion(item.Pos, item.Line, docs, ts)
	case token.Token_ENUM:
		p.parseEnum(item.Pos, item.Line, docs, ts)
	case token.Token_SCALAR:
		p.parseScalar(item.Pos, item.Line, docs, ts)
	case token.Token_DIRECTIVE:
		p.parseDirective(item.Pos, item.Line, docs, ts)
	case token.Token_SCHEMA:
		p.parseSchema(item.Pos, item.Line, docs, ts)
	default:
		p.errorf("unknown type")
	}
}

func (p *parser) parseDirectives(directives *[]*ast.DirectiveLit) {
	for {
		item := p.next() // This should always be served out of p.pk
		if item.Typ == token.Token_ERR {
			p.unexpected(item, "parseDirectives")
		}
		if item.Typ == token.Token_EOF {
			*directives = append(*directives, p.direcs...)
			p.direcs = p.direcs[:0]
			p.pk = item
			return
		}

		name := p.expect(token.Token_IDENT, "parseDirectives:MustHaveName")

		dir := &ast.DirectiveLit{
			AtPos: int64(item.Pos),
			Name:  name.Val,
		}
		p.direcs = append(p.direcs, dir)

		item = p.peek()
		if item.Typ == token.Token_LPAREN {
			dir.Args = &ast.CallExpr{
				Lparen: int64(item.Pos),
			}
			p.ignore()

			for {
				item = p.next()
				if item.Typ == token.Token_ERR || item.Typ == token.Token_EOF {
					p.unexpected(item, "parseDirectives:MalformedArg")
				}
				if item.Typ == token.Token_RPAREN {
					dir.Args.Args = append(dir.Args.Args, p.dargs...)
					p.dargs = p.dargs[:0]
					break
				}
				if item.Typ == token.Token_COMMENT && p.mode&ParseComments != 0 {
					continue // TODO
				}

				if item.Typ != token.Token_IDENT {
					p.unexpected(item, "parseDirectives:MissingArgName")
				}

				arg := &ast.Arg{
					Name: &ast.Ident{NamePos: int64(item.Pos), Name: item.Val},
				}
				p.expect(token.Token_COLON, "parseDirectives:MissingColon")

				val := p.parseValue()
				switch v := val.(type) {
				case *ast.BasicLit:
					arg.Value = &ast.Arg_BasicLit{BasicLit: v}
				case *ast.CompositeLit:
					arg.Value = &ast.Arg_CompositeLit{CompositeLit: v}
				}

				p.dargs = append(p.dargs, arg)
			}

			dir.Args.Rparen = int64(item.Pos)

			item = p.peek()
		}

		if item.Typ != token.Token_AT || item.Line != p.line { // Enforce directives being on the same line
			*directives = append(*directives, p.direcs...)
			p.direcs = p.direcs[:0]
			return
		}
	}
}

func (p *parser) parseObject(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
	name := p.expect(token.Token_IDENT, "parseObject:MustHaveName")

	ts.Name = &ast.Ident{
		NamePos: int64(name.Pos),
		Name:    name.Val,
	}
	obj := &ast.ObjectType{
		Object: int64(pos),
	}
	ts.Type = &ast.TypeSpec_Object{Object: obj}

	item := p.peek()
	if item.Typ == token.Token_IMPLEMENTS {
		p.ignore()
		obj.ImplPos = int64(item.Pos)

		for {
			item = p.peek()
			if item.Typ != token.Token_IDENT && item.Typ != token.Token_AND {
				break
			}
			if item.Typ == token.Token_AND {
				p.ignore()
				continue
			}

			obj.Interfaces = append(obj.Interfaces, &ast.Ident{NamePos: int64(item.Pos), Name: item.Val})
		}
	}

	if item.Typ == token.Token_AT {
		p.parseDirectives(&ts.Directives)
		item = p.pk
	}

	if item.Typ != token.Token_LBRACE {
		return
	}
	p.ignore()

	obj.Fields = &ast.FieldList{
		Opening: int64(item.Pos),
	}

	obj.Fields.Closing = p.parseFields(docs, &obj.Fields.List)
}

func (p *parser) parseInput(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
	name := p.expect(token.Token_IDENT, "parseInput:MustHaveName")

	ts.Name = &ast.Ident{
		NamePos: int64(name.Pos),
		Name:    name.Val,
	}
	input := &ast.InputType{
		Input: int64(pos),
	}
	ts.Type = &ast.TypeSpec_Input{Input: input}

	item := p.peek()
	if item.Typ == token.Token_AT {
		p.parseDirectives(&ts.Directives)
		item = p.pk
	}

	if item.Typ != token.Token_LBRACE {
		return
	}
	p.ignore()

	input.Fields = &ast.InputValueList{
		Opening: int64(item.Pos),
	}
	input.Fields.Closing = p.parseArgDefs(docs, &input.Fields.List)
}

func (p *parser) parseInterface(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
	name := p.expect(token.Token_IDENT, "parseInterface:MustHaveName")

	ts.Name = &ast.Ident{
		NamePos: int64(name.Pos),
		Name:    name.Val,
	}
	inter := &ast.InterfaceType{
		Interface: int64(pos),
	}
	ts.Type = &ast.TypeSpec_Interface{Interface: inter}

	item := p.peek()
	if item.Typ == token.Token_AT {
		p.parseDirectives(&ts.Directives)
		item = p.pk
	}

	if item.Typ != token.Token_LBRACE {
		return
	}
	p.ignore()

	inter.Fields = &ast.FieldList{
		Opening: int64(item.Pos),
	}
	inter.Fields.Closing = p.parseFields(docs, &inter.Fields.List)
}

func (p *parser) parseUnion(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
	name := p.expect(token.Token_IDENT, "parseUnion:MustHaveName")

	ts.Name = &ast.Ident{
		NamePos: int64(name.Pos),
		Name:    name.Val,
	}
	union := &ast.UnionType{
		Union: int64(pos),
	}
	ts.Type = &ast.TypeSpec_Union{Union: union}

	item := p.peek()
	if item.Typ == token.Token_AT {
		p.parseDirectives(&ts.Directives)
		item = p.pk
	}

	if item.Typ != token.Token_ASSIGN {
		return
	}
	p.ignore()

	for {
		item = p.peek()
		if item.Typ != token.Token_IDENT && item.Typ != token.Token_OR {
			return
		}
		if item.Typ == token.Token_OR {
			continue
		}

		union.Members = append(union.Members, &ast.Ident{NamePos: int64(item.Pos), Name: item.Val})
	}
}

func (p *parser) parseEnum(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
	name := p.expect(token.Token_IDENT, "parseEnum:MustHaveName")

	ts.Name = &ast.Ident{
		NamePos: int64(name.Pos),
		Name:    name.Val,
	}
	enum := &ast.EnumType{
		Enum: int64(pos),
	}
	ts.Type = &ast.TypeSpec_Enum{Enum: enum}

	item := p.peek()
	if item.Typ == token.Token_AT {
		p.parseDirectives(&ts.Directives)
		item = p.pk
	}

	if item.Typ != token.Token_LBRACE {
		return
	}
	p.ignore()

	enum.Values = &ast.FieldList{
		Opening: int64(item.Pos),
	}
	enum.Values.Closing = p.parseEnumValues(docs, &enum.Values.List)
}

func (p *parser) parseScalar(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
	name := p.expect(token.Token_IDENT, "parseScalar:MustHaveName")

	ts.Name = &ast.Ident{
		NamePos: int64(name.Pos),
		Name:    name.Val,
	}

	ts.Type = &ast.TypeSpec_Scalar{
		Scalar: &ast.ScalarType{
			Scalar: int64(pos),
			Name:   ts.Name,
		},
	}

	item := p.peek()
	if item.Typ == token.Token_AT && item.Line == line {
		p.parseDirectives(&ts.Directives)
	}
}

func (p *parser) parseDirective(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
	p.expect(token.Token_AT, "parseDirective")
	name := p.next()
	if name.Typ != token.Token_IDENT && !name.Typ.IsKeyword() {
		p.unexpected(name, "parseDirective:MustHaveName")
	}

	ts.Name = &ast.Ident{
		NamePos: int64(name.Pos),
		Name:    name.Val,
	}
	directive := &ast.DirectiveType{
		Directive: int64(pos),
	}
	ts.Type = &ast.TypeSpec_Directive{Directive: directive}

	item := p.next()
	if item.Typ == token.Token_LPAREN {
		directive.Args = &ast.InputValueList{
			Opening: int64(item.Pos),
		}
		directive.Args.Closing = p.parseArgDefs(docs, &directive.Args.List)
		item = p.next()
	}

	if item.Typ != token.Token_ON {
		p.unexpected(item, "parseDirective:MissingOnKeyword")
	}
	directive.OnPos = int64(item.Pos)

	for {
		item = p.peek()
		if item.Typ != token.Token_IDENT && item.Typ != token.Token_OR {
			return
		}
		if item.Typ == token.Token_OR {
			continue
		}

		loc, valid := ast.DirectiveLocation_Loc_value[item.Val]
		if !valid {
			p.errorf("invalid directive location: %s", item.Val)
		}

		directive.Locs = append(directive.Locs, &ast.DirectiveLocation{
			Start: int64(item.Pos),
			Loc:   ast.DirectiveLocation_Loc(loc),
		})
	}
}

func (p *parser) parseSchema(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
	schema := &ast.SchemaType{
		Schema: int64(pos),
	}
	ts.Type = &ast.TypeSpec_Schema{Schema: schema}

	item := p.peek()
	if item.Typ == token.Token_AT {
		p.parseDirectives(&ts.Directives)
		item = p.pk
	}

	if item.Typ != token.Token_LBRACE {
		return
	}
	p.ignore()

	schema.RootOps = &ast.FieldList{
		Opening: int64(item.Pos),
	}
	schema.RootOps.Closing = p.parseFields(docs, &schema.RootOps.List)
}

func (p *parser) parseFields(docs *[]*ast.DocGroup_Doc, fields *[]*ast.Field) int64 {
	for {
		item := p.next()
		switch {
		case item.Typ == token.Token_RBRACE:
			*docs = append(*docs, p.dg...)
			p.dg = p.dg[:0]

			*fields = append(*fields, p.fields...)
			p.fields = p.fields[:0]
			return int64(item.Pos)
		case item.Typ == token.Token_IDENT || item.Typ.IsKeyword():
			f := &ast.Field{
				Name: &ast.Ident{NamePos: int64(item.Pos), Name: item.Val},
			}
			p.fields = append(p.fields, f)

			item = p.peek()
			if item.Typ == token.Token_LPAREN {
				p.ignore()
				f.Args = &ast.InputValueList{
					Opening: int64(item.Pos),
				}

				f.Args.Closing = p.parseArgDefs(&p.dg, &p.fargs)
				if aLen := len(p.fargs); aLen > 0 {
					f.Args.List = make([]*ast.InputValue, aLen)
					copy(f.Args.List, p.fargs)
					p.fargs = p.fargs[:0]
				}

				item = p.peek()
			}
			if item.Typ != token.Token_COLON {
				p.unexpected(item, "parseFields:ExpectedColon")
			}
			p.ignore()

			if dLen := len(p.dg); dLen > 0 {
				f.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, dLen)}
				copy(f.Doc.List, p.dg)
				p.dg = p.dg[:0]
			}

			typ := p.parseType()
			switch v := typ.(type) {
			case *ast.Ident:
				f.Type = &ast.Field_Ident{Ident: v}
			case *ast.List:
				f.Type = &ast.Field_List{List: v}
			case *ast.NonNull:
				f.Type = &ast.Field_NonNull{NonNull: v}
			}

			p.pk = p.next()
			if p.pk.Typ != token.Token_AT {
				break
			}
			p.parseDirectives(&f.Directives)
		case item.Typ == token.Token_COMMENT && p.mode&ParseComments != 0 || item.Typ == token.Token_DESCRIPTION:
			d := &ast.DocGroup_Doc{
				Text:    item.Val,
				Char:    int64(item.Pos),
				Comment: item.Typ == token.Token_COMMENT,
			}

			if len(p.dg) == 0 {
				p.dg = append(p.dg, d)
				break
			}

			prev := p.dg[len(p.dg)-1]
			lprev := p.doc.Line(token.Pos(int(prev.Char) + len(prev.Text)))
			if p.doc.Line(token.Pos(d.Char))-lprev == 1 {
				p.dg = append(p.dg, d)
				break
			}

			*docs = append(*docs, p.dg...)
			p.dg = p.dg[:0]
			p.dg = append(p.dg, d)
		default:
			p.unexpected(item, "parseFields")
		}
	}
}

func (p *parser) parseArgDefs(docs *[]*ast.DocGroup_Doc, args *[]*ast.InputValue) int64 {
	for {
		item := p.next()
		switch {
		case item.Typ == token.Token_RPAREN || item.Typ == token.Token_RBRACE:
			*docs = append(*docs, p.cdg...)
			p.cdg = p.cdg[:0]

			*args = append(*args, p.args...)
			p.args = p.args[:0]
			return int64(item.Pos)
		case item.Typ == token.Token_IDENT || item.Typ.IsKeyword():
			arg := &ast.InputValue{
				Name: &ast.Ident{NamePos: int64(item.Pos), Name: item.Val},
			}
			p.args = append(p.args, arg)

			if dLen := len(p.cdg); dLen > 0 {
				arg.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, dLen)}
				copy(arg.Doc.List, p.cdg)
				p.cdg = p.cdg[:0]
			}

			p.expect(token.Token_COLON, "parseArgDefs:ExpectedColon")

			typ := p.parseType()
			switch v := typ.(type) {
			case *ast.Ident:
				arg.Type = &ast.InputValue_Ident{Ident: v}
			case *ast.List:
				arg.Type = &ast.InputValue_List{List: v}
			case *ast.NonNull:
				arg.Type = &ast.InputValue_NonNull{NonNull: v}
			}

			p.pk = p.next()
			if p.pk.Typ == token.Token_ASSIGN {
				p.ignore()

				val := p.parseValue()
				switch v := val.(type) {
				case *ast.BasicLit:
					arg.Default = &ast.InputValue_BasicLit{BasicLit: v}
				case *ast.CompositeLit:
					arg.Default = &ast.InputValue_CompositeLit{CompositeLit: v}
				}

				p.pk = p.next()
			}

			if p.pk.Typ != token.Token_AT {
				break
			}
			p.parseDirectives(&arg.Directives)
		case item.Typ == token.Token_COMMENT && p.mode&ParseComments != 0 || item.Typ == token.Token_DESCRIPTION:
			d := &ast.DocGroup_Doc{
				Text:    item.Val,
				Char:    int64(item.Pos),
				Comment: item.Typ == token.Token_COMMENT,
			}

			if len(p.cdg) == 0 {
				p.cdg = append(p.cdg, d)
				break
			}

			prev := p.cdg[len(p.cdg)-1]
			lprev := p.doc.Line(token.Pos(int(prev.Char) + len(prev.Text)))
			if p.doc.Line(token.Pos(d.Char))-lprev == 1 {
				p.cdg = append(p.cdg, d)
				break
			}

			*docs = append(*docs, p.cdg...)
			p.cdg = p.cdg[:0]
			p.cdg = append(p.cdg, d)
		default:
			p.unexpected(item, "parseArgDefs")
		}
	}
}

func (p *parser) parseEnumValues(docs *[]*ast.DocGroup_Doc, values *[]*ast.Field) int64 {
	for {
		item := p.next()
		switch {
		case item.Typ == token.Token_RBRACE:
			*docs = append(*docs, p.dg...)
			p.dg = p.dg[:0]

			*values = append(*values, p.fields...)
			p.fields = p.fields[:0]
			return int64(item.Pos)
		case item.Typ == token.Token_IDENT || item.Typ.IsKeyword():
			f := &ast.Field{
				Name: &ast.Ident{NamePos: int64(item.Pos), Name: item.Val},
			}
			p.fields = append(p.fields, f)

			if dLen := len(p.dg); dLen > 0 {
				f.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, dLen)}
				copy(f.Doc.List, p.dg)
				p.dg = p.dg[:0]
			}

			item = p.peek()
			if item.Typ == token.Token_AT {
				p.parseDirectives(&f.Directives)
			}
		case item.Typ == token.Token_COMMENT && p.mode&ParseComments != 0 || item.Typ == token.Token_DESCRIPTION:
			d := &ast.DocGroup_Doc{
				Text:    item.Val,
				Char:    int64(item.Pos),
				Comment: item.Typ == token.Token_COMMENT,
			}

			if len(p.dg) == 0 {
				p.dg = append(p.dg, d)
				break
			}

			prev := p.dg[len(p.dg)-1]
			lprev := p.doc.Line(token.Pos(int(prev.Char) + len(prev.Text)))
			if p.doc.Line(token.Pos(d.Char))-lprev == 1 {
				p.dg = append(p.dg, d)
				break
			}

			*docs = append(*docs, p.dg...)
			p.dg = p.dg[:0]
			p.dg = append(p.dg, d)
		default:
			p.unexpected(item, "parseEnumValues")
		}
	}
}

func (p *parser) parseType() interface{} {
	item := p.next()
	switch item.Typ {
	case token.Token_IDENT:
		v := &ast.Ident{NamePos: int64(item.Pos), Name: item.Val}

		item = p.peek()
		if item.Typ != token.Token_NOT {
			return v
		}
		p.ignore()

		return &ast.NonNull{
			Type: &ast.NonNull_Ident{Ident: v},
		}
	case token.Token_LBRACK:
		v := &ast.List{}

		typ := p.parseType()
		switch t := typ.(type) {
		case *ast.Ident:
			v.Type = &ast.List_Ident{Ident: t}
		case *ast.List:
			v.Type = &ast.List_List{List: t}
		case *ast.NonNull:
			v.Type = &ast.List_NonNull{NonNull: t}
		}

		item = p.next()
		if item.Typ != token.Token_RBRACK {
			p.unexpected(item, "parseType:MissingListRBrack")
		}

		item = p.peek()
		if item.Typ != token.Token_NOT {
			return v
		}
		p.ignore()

		return &ast.NonNull{
			Type: &ast.NonNull_List{List: v},
		}
	default:
		p.unexpected(item, "parseType")
	}
	return nil
}

func (p *parser) parseValue() interface{} {
	item := p.next()

	switch item.Typ {
	case token.Token_INT, token.Token_FLOAT, token.Token_STRING, token.Token_BOOL, token.Token_NULL, token.Token_IDENT:
		return &ast.BasicLit{Kind: item.Typ, ValuePos: int64(item.Pos), Value: item.Val}
	case token.Token_LBRACK:
		list := &ast.ListLit_Composite{}

		listLit := &ast.ListLit{List: &ast.ListLit_CompositeList{CompositeList: list}}
		v := &ast.CompositeLit{
			Opening: int64(item.Pos),
			Value:   &ast.CompositeLit_ListLit{ListLit: listLit},
		}

		var c *ast.CompositeLit
		for {
			item = p.peek()
			if item.Typ == token.Token_RBRACK {
				p.ignore()
				v.Closing = int64(item.Pos)
				return v
			}

			el := p.parseValue()
			switch e := el.(type) {
			case *ast.BasicLit:
				c = &ast.CompositeLit{Value: &ast.CompositeLit_BasicLit{BasicLit: e}}
			case *ast.CompositeLit:
				c = e
			}

			list.Values = append(list.Values, c)
		}
	case token.Token_LBRACE:
		objLit := new(ast.ObjLit)
		v := &ast.CompositeLit{
			Opening: int64(item.Pos),
			Value:   &ast.CompositeLit_ObjLit{ObjLit: objLit},
		}

		for {
			item = p.next()
			if item.Typ == token.Token_RBRACE {
				v.Closing = int64(item.Pos)
				return v
			}
			if item.Typ != token.Token_IDENT {
				p.unexpected(item, "parseValue:InvalidObjectKey")
			}

			pair := &ast.ObjLit_Pair{Key: &ast.Ident{NamePos: int64(item.Pos), Name: item.Val}}
			objLit.Fields = append(objLit.Fields, pair)
			p.expect(token.Token_COLON, "parseValue:MissingColonInObjField")

			val := p.parseValue()
			switch ov := val.(type) {
			case *ast.BasicLit:
				pair.Val = &ast.CompositeLit{Value: &ast.CompositeLit_BasicLit{BasicLit: ov}}
			case *ast.CompositeLit:
				pair.Val = ov
			}
		}
	default:
		p.unexpected(item, "parseValue")
	}
	return nil
}
