package parser

import (
	"fmt"
	"github.com/gqlc/graphql/ast"
	"github.com/gqlc/graphql/lexer"
	"github.com/gqlc/graphql/token"
	"io"
	"io/ioutil"
	"runtime"
)

// ParseDoc parses a single GraphQL Document.
func ParseDoc2(dset *token.DocSet, name string, src io.Reader, mode Mode) (*ast.Document, error) {
	// Assume src isn't massive so we're gonna just read it all
	b, err := ioutil.ReadAll(src)
	if err != nil {
		return nil, err
	}

	// Create parser and doc to doc set. Then, parse doc.
	d := dset.AddDoc(name, -1, len(b))
	p := &parser2{name: name}

	return p.parse(d, b, mode)
}

type parser2 struct {
	doc  *token.Doc
	l    lexer.Interface
	name string
	line int
	pk   lexer.Item
	mode Mode

	schema *ast.TypeDecl
}

// next returns the next token
func (p *parser2) next() (i lexer.Item) {
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
func (p *parser2) peek() lexer.Item {
	p.pk = p.l.NextItem()
	return p.pk
}

func (p *parser2) ignore() { p.pk = lexer.Item{} }

// expect consumes the next token and guarantees it has the required type.
func (p *parser2) expect(tok token.Token, context string) lexer.Item {
	i := p.l.NextItem()
	if i.Typ != tok {
		p.unexpected(i, context)
	}
	return i
}

// errorf formats the error and terminates processing.
func (p *parser2) errorf(format string, args ...interface{}) {
	format = fmt.Sprintf("parser: %s:%d: %s", p.name, p.line, format)
	panic(fmt.Errorf(format, args...))
}

// error terminates processing.
func (p *parser2) error(err error) {
	p.errorf("%s", err)
}

// unexpected complains about the token and terminates processing.
func (p *parser2) unexpected(token lexer.Item, context string) {
	p.errorf("unexpected %s in %s", token, context)
}

// recover is the handler that turns panics into returns from the top level of parse.
func (p *parser2) recover(err *error) {
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

func (p *parser2) parse(tokDoc *token.Doc, b []byte, mode Mode) (doc *ast.Document, err error) {
	defer p.recover(&err)
	p.l = lexer.Lex(tokDoc, string(b))
	p.doc = tokDoc

	doc = &ast.Document{
		Name: p.name,
	}
	p.parseDoc(doc.Doc, &doc.Types, &doc.Directives)

	if p.schema != nil {
		doc.Schema = p.schema
	}
	return
}

func (p *parser2) parseDoc(pdg *ast.DocGroup, types *[]*ast.TypeDecl, directives *[]*ast.DirectiveLit) {
	var docs []*ast.DocGroup_Doc
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
			p.parseDef(typ, &docs, ts)

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
			if dLen := len(docs); dLen > 0 {
				td.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, dLen)}
				copy(td.Doc.List, docs)
				docs = docs[:0]
			}

			*types = append(*types, td)
		case item.Typ.IsKeyword():
			ts.Reset()

			p.parseDef(item, &docs, ts)

			tts := *ts
			td := &ast.TypeDecl{
				TokPos: int64(item.Pos),
				Tok:    item.Typ,
				Spec:   &ast.TypeDecl_TypeSpec{TypeSpec: &tts},
			}
			if dLen := len(docs); dLen > 0 {
				td.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, dLen)}
				copy(td.Doc.List, docs)
				docs = docs[:0]
			}

			*types = append(*types, td)

			if item.Typ == token.Token_SCHEMA {
				p.schema = td
			}
		case item.Typ == token.Token_COMMENT && p.mode&ParseComments != 0 || item.Typ == token.Token_DESCRIPTION:
			d := &ast.DocGroup_Doc{
				Text: item.Val,
				Char: int64(item.Pos),
			}

			if item.Typ == token.Token_COMMENT {
				d.Comment = true
			}

			if len(docs) == 0 {
				docs = append(docs, d)
				break
			}

			prev := docs[len(docs)-1]
			lprev := p.doc.Line(token.Pos(int(prev.Char) + len(prev.Text)))
			if p.doc.Line(token.Pos(d.Char))-lprev == 1 {
				docs = append(docs, d)
				break
			}

			pdg.List = append(pdg.List, docs...)
			docs = docs[:0]
			docs = append(docs, d)
		case item.Typ == token.Token_AT:
			p.pk = item
			p.parseDirectives(directives)
		case item.Typ == token.Token_COMMENT:
		default:
			p.unexpected(item, "parseDoc:UnknownToken")
		}
	}
}

func (p *parser2) parseDef(item lexer.Item, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
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

func (p *parser2) parseDirectives(directives *[]*ast.DirectiveLit) {
	for {
		item := p.next() // This should always be served out of p.pk
		if item.Typ == token.Token_ERR {
			p.unexpected(item, "parseDirectives")
		}
		if item.Typ == token.Token_EOF {
			p.pk = item
			return
		}

		name := p.expect(token.Token_IDENT, "parseDirectives:MustHaveName")

		dir := &ast.DirectiveLit{
			AtPos: int64(item.Pos),
			Name:  name.Val,
		}
		*directives = append(*directives, dir)

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

				dir.Args.Args = append(dir.Args.Args, arg)
			}

			dir.Args.Rparen = int64(item.Pos)

			item = p.peek()
		}

		if item.Typ != token.Token_AT || item.Line != p.line { // Enforce directives being on the same line
			return
		}
	}
}

func (p *parser2) parseObject(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
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

func (p *parser2) parseInput(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
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

func (p *parser2) parseInterface(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
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

func (p *parser2) parseUnion(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
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

func (p *parser2) parseEnum(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
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

func (p *parser2) parseScalar(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
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

func (p *parser2) parseDirective(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
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

func (p *parser2) parseSchema(pos token.Pos, line int, docs *[]*ast.DocGroup_Doc, ts *ast.TypeSpec) {
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

func (p *parser2) parseFields(docs *[]*ast.DocGroup_Doc, fields *[]*ast.Field) int64 {
	var fdocs []*ast.DocGroup_Doc
	var args []*ast.InputValue
	for {
		item := p.next()
		switch {
		case item.Typ == token.Token_RBRACE:
			return int64(item.Pos)
		case item.Typ == token.Token_IDENT || item.Typ.IsKeyword():
			f := &ast.Field{
				Name: &ast.Ident{NamePos: int64(item.Pos), Name: item.Val},
			}
			*fields = append(*fields, f)

			if dLen := len(fdocs); dLen > 0 {
				f.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, dLen)}
				copy(f.Doc.List, fdocs)
				fdocs = fdocs[:0]
			}

			item = p.peek()
			if item.Typ == token.Token_LPAREN {
				p.ignore()
				f.Args = &ast.InputValueList{
					Opening: int64(item.Pos),
				}

				f.Args.Closing = p.parseArgDefs(&fdocs, &args)
				if aLen := len(args); aLen > 0 {
					f.Args.List = make([]*ast.InputValue, aLen)
					copy(f.Args.List, args)
					args = args[:0]
				}

				item = p.peek()
			}
			if item.Typ != token.Token_COLON {
				p.unexpected(item, "parseFields:ExpectedColon")
			}
			p.ignore()

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
				Text: item.Val,
				Char: int64(item.Pos),
			}

			if item.Typ == token.Token_COMMENT {
				d.Comment = true
			}

			if len(fdocs) == 0 {
				fdocs = append(fdocs, d)
				break
			}

			prev := fdocs[len(fdocs)-1]
			lprev := p.doc.Line(token.Pos(int(prev.Char) + len(prev.Text)))
			if p.doc.Line(token.Pos(d.Char))-lprev == 1 {
				fdocs = append(fdocs, d)
				break
			}

			*docs = append(*docs, fdocs...)
			fdocs = fdocs[:0]
			fdocs = append(fdocs, d)
		default:
			p.unexpected(item, "parseFields")
		}
	}
}

func (p *parser2) parseArgDefs(docs *[]*ast.DocGroup_Doc, args *[]*ast.InputValue) int64 {
	var adocs []*ast.DocGroup_Doc
	for {
		item := p.next()
		switch {
		case item.Typ == token.Token_RPAREN || item.Typ == token.Token_RBRACE:
			return int64(item.Pos)
		case item.Typ == token.Token_IDENT || item.Typ.IsKeyword():
			arg := &ast.InputValue{
				Name: &ast.Ident{NamePos: int64(item.Pos), Name: item.Val},
			}
			*args = append(*args, arg)

			if dLen := len(adocs); dLen > 0 {
				arg.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, dLen)}
				copy(arg.Doc.List, adocs)
				adocs = adocs[:0]
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
				Text: item.Val,
				Char: int64(item.Pos),
			}

			if item.Typ == token.Token_COMMENT {
				d.Comment = true
			}

			if len(adocs) == 0 {
				adocs = append(adocs, d)
				break
			}

			prev := adocs[len(adocs)-1]
			lprev := p.doc.Line(token.Pos(int(prev.Char) + len(prev.Text)))
			if p.doc.Line(token.Pos(d.Char))-lprev == 1 {
				adocs = append(adocs, d)
				break
			}

			*docs = append(*docs, adocs...)
			adocs = adocs[:0]
			adocs = append(adocs, d)
		default:
			p.unexpected(item, "parseArgDefs")
		}
	}
}

func (p *parser2) parseEnumValues(docs *[]*ast.DocGroup_Doc, values *[]*ast.Field) int64 {
	var fdocs []*ast.DocGroup_Doc
	var args []*ast.InputValue
	for {
		item := p.next()
		switch {
		case item.Typ == token.Token_RBRACE:
			return int64(item.Pos)
		case item.Typ == token.Token_IDENT || item.Typ.IsKeyword():
			f := &ast.Field{
				Name: &ast.Ident{NamePos: int64(item.Pos), Name: item.Val},
			}
			*values = append(*values, f)

			if dLen := len(fdocs); dLen > 0 {
				f.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, dLen)}
				copy(f.Doc.List, fdocs)
				fdocs = fdocs[:0]
			}

			item = p.peek()
			if item.Typ == token.Token_LPAREN {
				p.ignore()
				f.Args = &ast.InputValueList{
					Opening: int64(item.Pos),
				}

				f.Args.Closing = p.parseArgDefs(&fdocs, &args)
				if aLen := len(args); aLen > 0 {
					f.Args.List = make([]*ast.InputValue, aLen)
					copy(f.Args.List, args)
					args = args[:0]
				}

				item = p.peek()
			}

			if item.Typ == token.Token_AT {
				p.parseDirectives(&f.Directives)
			}
		case item.Typ == token.Token_COMMENT && p.mode&ParseComments != 0 || item.Typ == token.Token_DESCRIPTION:
			d := &ast.DocGroup_Doc{
				Text: item.Val,
				Char: int64(item.Pos),
			}

			if item.Typ == token.Token_COMMENT {
				d.Comment = true
			}

			if len(fdocs) == 0 {
				fdocs = append(fdocs, d)
				break
			}

			prev := fdocs[len(fdocs)-1]
			lprev := p.doc.Line(token.Pos(int(prev.Char) + len(prev.Text)))
			if p.doc.Line(token.Pos(d.Char))-lprev == 1 {
				fdocs = append(fdocs, d)
				break
			}

			*docs = append(*docs, fdocs...)
			fdocs = fdocs[:0]
			fdocs = append(fdocs, d)
		default:
			p.unexpected(item, "parseEnumValues")
		}
	}
}

func (p *parser2) parseType() interface{} {
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

func (p *parser2) parseValue() interface{} {
	item := p.next()

	switch item.Typ {
	case token.Token_INT, token.Token_FLOAT, token.Token_STRING, token.Token_BOOL:
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
