// Package parser implements a parser for GraphQL IDL source files.
//
package parser

import (
	"fmt"
	"github.com/gqlc/graphql/ast"
	"github.com/gqlc/graphql/lexer"
	"github.com/gqlc/graphql/token"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
)

// Mode represents a parsing mode.
type Mode uint

// Mode Options
const (
	ParseComments = 1 << iota // parse comments and add them to the schema
)

// ParseDir calls ParseDoc for all files with names ending in ".gql"/".graphql" in the
// directory specified by path and returns a map of document name -> *ast.Document for all
// the documents found.
//
func ParseDir(dset *token.DocSet, path string, filter func(os.FileInfo) bool, mode Mode) (docs map[string]*ast.Document, err error) {
	if filter == nil {
		filter = func(os.FileInfo) bool { return false }
	}

	docs = make(map[string]*ast.Document)
	err = filepath.Walk(path, func(p string, info os.FileInfo, e error) error {
		skip := filter(info)
		if skip && info.IsDir() {
			return filepath.SkipDir
		}

		ext := filepath.Ext(p)
		if skip || info.IsDir() || ext != ".gql" && ext != ".graphql" {
			return nil
		}

		f, err := os.Open(p)
		if err != nil {
			return err
		}

		doc, err := ParseDoc(dset, info.Name(), f, mode)
		f.Close() // TODO: Handle this error
		if err != nil {
			return err
		}

		docs[doc.Name] = doc
		return nil
	})
	return
}

// ParseDoc parses a single GraphQL Document.
//
func ParseDoc(dset *token.DocSet, name string, src io.Reader, mode Mode) (*ast.Document, error) {
	// Assume src isn't massive so we're gonna just read it all
	b, err := ioutil.ReadAll(src)
	if err != nil {
		return nil, err
	}

	// Create parser and doc to doc set. Then, parse doc.
	p := &parser{name: name}
	d := dset.AddDoc(name, -1, len(b))
	return p.parse(d, b, mode)
}

// ParseDocs parses a set of GraphQL documents. Any import paths
// in a doc will be resolved against the provided doc names in the docs map.
//
func ParseDocs(dset *token.DocSet, docs map[string]io.Reader, mode Mode) ([]*ast.Document, error) {
	odocs := make([]*ast.Document, len(docs))

	i := 0
	for name, src := range docs {
		doc, err := ParseDoc(dset, name, src, mode)
		if err != nil {
			return odocs, err
		}
		odocs[i] = doc
		i++
	}
	return odocs, nil
}

type parser struct {
	l    lexer.Interface
	name string
	line int
	pk   lexer.Item
	mode Mode
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

// ErrUnexpectedItem represents encountering an unexpected item from the lexer.
type ErrUnexpectedItem struct {
	i lexer.Item
}

// Error formats an ErrUnexpectedItem error.
func (e ErrUnexpectedItem) Error() string {
	return fmt.Sprintf("unexpected token encountered- line: %d, pos: %d, type: %s, value: %s", e.i.Line, e.i.Pos, e.i.Typ, e.i.String())
}

// parse parses a GraphQL document
func (p *parser) parse(doc *token.Doc, src []byte, mode Mode) (d *ast.Document, err error) {
	p.l = lexer.Lex(doc, string(src))
	p.mode = mode

	d = &ast.Document{
		Name: doc.Name(),
		Doc:  new(ast.DocGroup),
	}
	defer p.recover(&err)
	p.parseDoc(d.Doc, d)
	return d, nil
}

// addDocs slurps up documentation
func (p *parser) addDocs(pdg *ast.DocGroup) (cdg *ast.DocGroup, item lexer.Item) {
	cdg = new(ast.DocGroup)
	for {
		// Get next item
		item = p.next()
		isComment := item.Typ == token.COMMENT
		if !isComment && item.Typ != token.DESCRIPTION {
			p.pk = lexer.Item{}
			return
		}

		// Skip a comment if they're not being parsed
		if isComment && p.mode&ParseComments == 0 {
			continue
		}
		cdg.List = append(cdg.List, &ast.DocGroup_Doc{
			Text:    item.Val,
			Char:    int64(item.Pos),
			Comment: isComment,
		})

		// Peek next item.
		nitem := p.next()
		lineDiff := nitem.Line - item.Line
		if lineDiff < 2 {
			p.pk = nitem
			continue
		}

		// Add cdg to pdg
		pdg.List = append(pdg.List, cdg.List...)
	}
}

// parseDoc parses a GraphQL document
func (p *parser) parseDoc(dg *ast.DocGroup, d *ast.Document) {
	// Slurp up documentation
	cdg, item := p.addDocs(dg)

	switch item.Typ {
	case token.ERR:
		p.unexpected(item, "parseDoc")
	case token.EOF:
		return
	case token.AT:
		p.parseDirectiveLit(cdg, item, &d.Directives)
	case token.SCHEMA:
		p.parseSchema(item, cdg, d)
		d.Schema = d.Types[len(d.Types)-1]
	case token.SCALAR:
		p.parseScalar(item, cdg, d)
	case token.TYPE:
		p.parseObject(item, cdg, d)
	case token.INTERFACE:
		p.parseInterface(item, cdg, d)
	case token.UNION:
		p.parseUnion(item, cdg, d)
	case token.ENUM:
		p.parseEnum(item, cdg, d)
	case token.INPUT:
		p.parseInput(item, cdg, d)
	case token.DIRECTIVE:
		p.parseDirective(item, cdg, d)
	case token.EXTEND:
		p.parseExtension(item, cdg, d)
	}

	p.parseDoc(dg, d)
}

// parseSchema parses a schema declaration
func (p *parser) parseSchema(item lexer.Item, dg *ast.DocGroup, doc *ast.Document) {
	// Create schema general decl node
	schemaDecl := &ast.TypeDecl{
		Doc:    dg,
		Tok:    int64(token.SCHEMA),
		TokPos: int64(item.Pos),
	}
	doc.Types = append(doc.Types, schemaDecl)

	// Slurp up applied directives
	dirs, nitem := p.parseDirectives(dg)

	// Create schema type spec node
	schemaSpec := &ast.TypeSpec{
		Doc:        dg,
		Name:       nil,
		Directives: dirs,
	}
	schemaDecl.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: schemaSpec}

	// Create schema type node
	schemaTyp := &ast.SchemaType{
		Schema:  int64(item.Pos),
		RootOps: new(ast.FieldList),
	}
	schemaSpec.Type = &ast.TypeSpec_Schema{Schema: schemaTyp}

	if nitem.Typ != token.LBRACE {
		p.pk = nitem
		return
	}
	schemaTyp.RootOps.Opening = int64(nitem.Pos)

	for {
		cdg, fitem := p.addDocs(dg)
		if fitem.Typ == token.RBRACE {
			schemaTyp.RootOps.Closing = int64(fitem.Pos)
			return
		}

		if fitem.Typ != token.IDENT {
			p.unexpected(fitem, "parseSchema")
		}

		if fitem.Val != "query" && fitem.Val != "mutation" && fitem.Val != "subscription" {
			p.unexpected(fitem, "parseSchema")
		}

		f := &ast.Field{
			Doc: cdg,
			Name: &ast.Ident{
				NamePos: int64(fitem.Pos),
				Name:    fitem.Val,
			},
		}
		schemaTyp.RootOps.List = append(schemaTyp.RootOps.List, f)

		p.expect(token.COLON, "parseSchema")

		fitem = p.expect(token.IDENT, "parseSchema")
		f.Type = &ast.Field_Ident{Ident: &ast.Ident{
			NamePos: int64(fitem.Pos),
			Name:    fitem.Val,
		}}
	}
}

// parseDirectives parses a list of applied directives
func (p *parser) parseDirectives(dg *ast.DocGroup) (dirs []*ast.DirectiveLit, item lexer.Item) {
	for {
		item = p.next()
		if item.Typ != token.AT {
			return
		}

		p.parseDirectiveLit(dg, item, &dirs)
	}
}

func (p *parser) parseDirectiveLit(dg *ast.DocGroup, item lexer.Item, dirs *[]*ast.DirectiveLit) {
	dir := &ast.DirectiveLit{
		AtPos: int64(item.Pos),
	}
	*dirs = append(*dirs, dir)

	item = p.expect(token.IDENT, "parseDirectives")
	dir.Name = item.Val

	item = p.next()
	if item.Typ != token.LPAREN {
		p.pk = item
		return
	}

	args, rpos := p.parseArgs(dg)

	dir.Args = &ast.CallExpr{
		Lparen: int64(item.Pos),
		Args:   args,
		Rparen: int64(rpos),
	}
}

// parseArgs parses a list of arguments
func (p *parser) parseArgs(pdg *ast.DocGroup) (args []*ast.Arg, rpos token.Pos) {
	for {
		_, item := p.addDocs(pdg)
		if item.Typ == token.RPAREN {
			rpos = item.Pos
			return
		}

		if item.Typ != token.IDENT {
			p.unexpected(item, "parseArgs:ArgName")
		}
		a := &ast.Arg{
			Name: &ast.Ident{
				NamePos: int64(item.Pos),
				Name:    item.Val,
			},
		}
		args = append(args, a)

		p.expect(token.COLON, "parseArgs:MustHaveColon")

		iVal := p.parseValue()
		switch v := iVal.(type) {
		case *ast.BasicLit:
			a.Value = &ast.Arg_BasicLit{BasicLit: v}
		case *ast.CompositeLit:
			a.Value = &ast.Arg_CompositeLit{CompositeLit: v}
		}
	}
}

// parseValue parses a value
func (p *parser) parseValue() (v interface{}) {
	item := p.next()

	switch item.Typ {
	case token.COMMENT:
		return p.parseValue()
	case token.LBRACK:
		listLit := &ast.CompositeLit_ListLit{ListLit: &ast.ListLit{}}
		compLit := &ast.CompositeLit{
			Opening: int64(item.Pos),
			Value:   listLit,
		}
		v = compLit

		var i interface{}
		for {
			item = p.peek()
			if item.Typ == token.RBRACK {
				compLit.Closing = int64(item.Pos)
				p.pk = lexer.Item{}
				return
			}

			i = p.parseValue()
			switch t := i.(type) {
			case *ast.BasicLit:
				if listLit.ListLit.List == nil {
					listLit.ListLit.List = &ast.ListLit_BasicList{BasicList: &ast.ListLit_Basic{}}
				}

				bList, ok := listLit.ListLit.List.(*ast.ListLit_BasicList)
				if !ok {
					p.errorf("cannot mix list types")
					return
				}

				bList.BasicList.Values = append(bList.BasicList.Values, t)
			case *ast.CompositeLit:
				if listLit.ListLit.List == nil {
					listLit.ListLit.List = &ast.ListLit_CompositeList{CompositeList: &ast.ListLit_Composite{}}
				}

				cList, ok := listLit.ListLit.List.(*ast.ListLit_CompositeList)
				if !ok {
					p.errorf("cannot mix list types")
					return
				}

				cList.CompositeList.Values = append(cList.CompositeList.Values, t)
			}
		}
	case token.LBRACE:
		objLit := &ast.ObjLit{}
		compLit := &ast.CompositeLit{
			Opening: int64(item.Pos),
			Value:   &ast.CompositeLit_ObjLit{ObjLit: objLit},
		}
		v = compLit

		for {
			item = p.peek()
			if item.Typ == token.RBRACE {
				compLit.Closing = int64(item.Pos)
				p.pk = lexer.Item{}
				return
			}

			id := p.parseIdent("parseValue:ObjectLit")

			p.expect(token.COLON, "parseValue:ObjectLit")

			pcLit := &ast.CompositeLit{}
			i := p.parseValue()
			switch t := i.(type) {
			case *ast.BasicLit:
				pcLit.Value = &ast.CompositeLit_BasicLit{BasicLit: t}
			case *ast.CompositeLit:
				pcLit = t
			}

			objLit.Fields = append(objLit.Fields, &ast.ObjLit_Pair{
				Key: id,
				Val: pcLit,
			})
		}
	case token.STRING, token.INT, token.FLOAT, token.IDENT:
		// Enforce true/false for ident
		if item.Typ == token.IDENT {
			switch item.Val {
			case "true", "false":
				item.Typ = token.BOOL
			case "null":
				item.Typ = token.NULL
			}
		}

		// Construct basic literal
		v = &ast.BasicLit{
			ValuePos: int64(item.Pos),
			Value:    item.Val,
			Kind:     int64(item.Typ),
		}
		return
	default:
		p.unexpected(item, "parseValue")
	}
	return
}

func (p *parser) parseFieldDefs(pdg *ast.DocGroup) (fields []*ast.Field, rpos int64) {
	for {
		cdg, item := p.addDocs(pdg)
		if item.Typ == token.RBRACE {
			rpos = int64(item.Pos)
			return
		}

		if item.Typ != token.IDENT && !item.Typ.IsKeyword() {
			p.unexpected(item, "parseFieldDefs:MustHaveName")
		}
		f := &ast.Field{
			Doc: cdg,
			Name: &ast.Ident{
				NamePos: int64(item.Pos),
				Name:    item.Val,
			},
		}
		fields = append(fields, f)

		item = p.next()
		if item.Typ == token.LPAREN {
			f.Args = &ast.InputValueList{
				Opening: int64(item.Pos),
			}
			f.Args.List, f.Args.Closing = p.parseArgDefs(cdg)
			item = p.next()
		}
		if item.Typ != token.COLON {
			p.unexpected(item, "parseFieldDefs:MustHaveColon")
		}

		typ := p.parseType(p.next())
		switch v := typ.(type) {
		case *ast.Ident:
			f.Type = &ast.Field_Ident{Ident: v}
		case *ast.List:
			f.Type = &ast.Field_List{List: v}
		case *ast.NonNull:
			f.Type = &ast.Field_NonNull{NonNull: v}
		}

		f.Directives, p.pk = p.parseDirectives(pdg)
	}
}

// parseArgDefs parses a list of argument definitions.
func (p *parser) parseArgDefs(pdg *ast.DocGroup) (args []*ast.InputValue, rpos int64) {
	for {
		cdg, item := p.addDocs(pdg)
		if item.Typ == token.RPAREN || item.Typ == token.RBRACE {
			rpos = int64(item.Pos)
			return
		}

		if item.Typ != token.IDENT && !item.Typ.IsKeyword() {
			p.unexpected(item, "parseArgsDef:MustHaveName")
		}
		arg := &ast.InputValue{
			Doc: cdg,
			Name: &ast.Ident{
				NamePos: int64(item.Pos),
				Name:    item.Val,
			},
		}
		args = append(args, arg)

		p.expect(token.COLON, "parseArgDefs:MustHaveColon")

		typ := p.parseType(p.next())
		switch v := typ.(type) {
		case *ast.Ident:
			arg.Type = &ast.InputValue_Ident{Ident: v}
		case *ast.List:
			arg.Type = &ast.InputValue_List{List: v}
		case *ast.NonNull:
			arg.Type = &ast.InputValue_NonNull{NonNull: v}
		}

		item = p.next()
		if item.Typ == token.ASSIGN {
			p.pk = lexer.Item{}
			iVal := p.parseValue()
			switch v := iVal.(type) {
			case *ast.BasicLit:
				arg.Default = &ast.InputValue_BasicLit{BasicLit: v}
			case *ast.CompositeLit:
				arg.Default = &ast.InputValue_CompositeLit{CompositeLit: v}
			}
		} else {
			p.pk = item
		}

		arg.Directives, p.pk = p.parseDirectives(pdg)
	}
}

func (p *parser) parseType(item lexer.Item) (e interface{}) {
	switch item.Typ {
	case token.LBRACK:
		item = p.next()
		switch v := p.parseType(item).(type) {
		case *ast.Ident:
			e = &ast.List{
				Type: &ast.List_Ident{Ident: v},
			}
		case *ast.List:
			e = &ast.List{
				Type: &ast.List_List{List: v},
			}
		case *ast.NonNull:
			e = &ast.List{
				Type: &ast.List_NonNull{NonNull: v},
			}
		}

		item = p.next()
		if item.Typ != token.RBRACK {
			p.unexpected(item, "parseType:MustCloseListType")
		}

		item = p.next()
		if item.Typ != token.NOT {
			p.pk = item
			return
		}

		switch v := e.(type) {
		case *ast.Ident:
			e = &ast.NonNull{
				Type: &ast.NonNull_Ident{Ident: v},
			}
		case *ast.List:
			e = &ast.NonNull{
				Type: &ast.NonNull_List{List: v},
			}
		default:
			p.unexpected(item, "parseType:RepeatedNonNull")
		}
	case token.IDENT:
		p.pk = item
		e = p.parseIdent("parseType")

		item = p.next()
		if item.Typ != token.NOT {
			p.pk = item
			return
		}
		p.pk = lexer.Item{}

		switch v := e.(type) {
		case *ast.Ident:
			e = &ast.NonNull{
				Type: &ast.NonNull_Ident{Ident: v},
			}
		case *ast.List:
			e = &ast.NonNull{
				Type: &ast.NonNull_List{List: v},
			}
		default:
			p.unexpected(item, "parseType:RepeatedNonNull")
		}
	default:
		p.unexpected(item, "parseType")
	}
	return
}

// parseIdent parses an identifier
func (p *parser) parseIdent(context string) *ast.Ident {
	item := p.next()
	if item.Typ != token.IDENT {
		p.pk = item
		return nil
	}
	return &ast.Ident{
		NamePos: int64(item.Pos),
		Name:    item.Val,
	}
}

// parseScalar parses a scalar declaration
func (p *parser) parseScalar(item lexer.Item, dg *ast.DocGroup, doc *ast.Document) {
	scalarGen := &ast.TypeDecl{
		Doc:    dg,
		TokPos: int64(item.Pos),
		Tok:    int64(token.SCALAR),
	}
	doc.Types = append(doc.Types, scalarGen)

	name := p.parseIdent("parseScalar")

	scalarSpec := &ast.TypeSpec{
		Doc:  dg,
		Name: name,
	}
	scalarGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: scalarSpec}

	scalarSpec.Directives, p.pk = p.parseDirectives(dg)

	scalarType := &ast.ScalarType{
		Scalar: int64(item.Pos),
		Name:   scalarSpec.Name,
	}
	scalarSpec.Type = &ast.TypeSpec_Scalar{Scalar: scalarType}
}

// parseObject parses an object declaration
func (p *parser) parseObject(item lexer.Item, dg *ast.DocGroup, doc *ast.Document) {
	objGen := &ast.TypeDecl{
		Doc:    dg,
		TokPos: int64(item.Pos),
		Tok:    int64(token.TYPE),
	}
	doc.Types = append(doc.Types, objGen)

	name := p.parseIdent("parseObject")

	objSpec := &ast.TypeSpec{
		Doc:  dg,
		Name: name,
	}
	objGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: objSpec}

	objType := &ast.ObjectType{
		Object: int64(item.Pos),
	}
	objSpec.Type = &ast.TypeSpec_Object{Object: objType}

	item = p.next()
	if item.Typ == token.IMPLEMENTS {
		objType.ImplPos = int64(item.Pos)
		for {
			inter := p.parseIdent("parseObject:Interfaces")
			if inter == nil {
				break
			}

			objType.Interfaces = append(objType.Interfaces, inter)
		}
		item = p.next()
	}

	if item.Typ == token.AT {
		p.pk = item
		objSpec.Directives, item = p.parseDirectives(dg)
	}

	if item.Typ != token.LBRACE {
		p.pk = item
		return
	}
	p.pk = lexer.Item{}

	objType.Fields = &ast.FieldList{
		Opening: int64(item.Pos),
	}
	objType.Fields.List, objType.Fields.Closing = p.parseFieldDefs(dg)
}

// parseInterface parses an interface declaration
func (p *parser) parseInterface(item lexer.Item, dg *ast.DocGroup, doc *ast.Document) {
	interfaceGen := &ast.TypeDecl{
		Doc:    dg,
		TokPos: int64(item.Pos),
		Tok:    int64(token.INTERFACE),
	}
	doc.Types = append(doc.Types, interfaceGen)

	name := p.parseIdent("parseInterface")

	interfaceSpec := &ast.TypeSpec{
		Doc:  dg,
		Name: name,
	}
	interfaceGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: interfaceSpec}

	interfaceType := &ast.InterfaceType{
		Interface: int64(item.Pos),
	}
	interfaceSpec.Type = &ast.TypeSpec_Interface{Interface: interfaceType}

	item = p.next()
	if item.Typ == token.AT {
		p.pk = item
		interfaceSpec.Directives, item = p.parseDirectives(dg)
	}

	if item.Typ != token.LBRACE {
		p.pk = item
		return
	}
	p.pk = lexer.Item{}

	interfaceType.Fields = &ast.FieldList{
		Opening: int64(item.Pos),
	}
	interfaceType.Fields.List, interfaceType.Fields.Closing = p.parseFieldDefs(dg)
}

// parseUnion parses a union declaration
func (p *parser) parseUnion(item lexer.Item, dg *ast.DocGroup, doc *ast.Document) {
	uGen := &ast.TypeDecl{
		Doc:    dg,
		TokPos: int64(item.Pos),
		Tok:    int64(token.UNION),
	}
	doc.Types = append(doc.Types, uGen)

	name := p.parseIdent("parseUnion")

	uSpec := &ast.TypeSpec{
		Doc:  dg,
		Name: name,
	}
	uGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: uSpec}

	uType := &ast.UnionType{
		Union: int64(item.Pos),
	}
	uSpec.Type = &ast.TypeSpec_Union{Union: uType}

	item = p.next()
	if item.Typ == token.AT {
		p.pk = item
		uSpec.Directives, item = p.parseDirectives(dg)
	}

	if item.Typ != token.ASSIGN {
		p.pk = item
		return
	}

	for {
		p.pk = p.next()
		if p.pk.Typ == token.EOF {
			return
		}

		mem := p.parseIdent("parseUnion:Members")
		if mem == nil {
			return
		}
		uType.Members = append(uType.Members, mem)
	}
}

// parseEnum parses an enum declaration
func (p *parser) parseEnum(item lexer.Item, dg *ast.DocGroup, doc *ast.Document) {
	eGen := &ast.TypeDecl{
		Doc:    dg,
		TokPos: int64(item.Pos),
		Tok:    int64(token.ENUM),
	}
	doc.Types = append(doc.Types, eGen)

	name := p.parseIdent("parseEnum")

	eSpec := &ast.TypeSpec{
		Doc:  dg,
		Name: name,
	}
	eGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: eSpec}

	eType := &ast.EnumType{
		Enum: int64(item.Pos),
	}
	eSpec.Type = &ast.TypeSpec_Enum{Enum: eType}

	item = p.next()
	if item.Typ == token.AT {
		p.pk = item
		eSpec.Directives, item = p.parseDirectives(dg)
	}

	if item.Typ != token.LBRACE {
		p.pk = item
		return
	}
	p.pk = lexer.Item{}

	eType.Values = &ast.FieldList{
		Opening: int64(item.Pos),
	}
	for {
		fdg, item := p.addDocs(dg)
		if item.Typ == token.RBRACE {
			p.pk = item
			return
		}
		if item.Typ != token.IDENT {
			p.unexpected(item, "parseEnum:Field")
		}

		f := &ast.Field{
			Doc: fdg,
			Name: &ast.Ident{
				NamePos: int64(item.Pos),
				Name:    item.Val,
			},
		}
		eType.Values.List = append(eType.Values.List, f)

		item = p.peek()
		if item.Typ != token.AT {
			continue
		}
		f.Directives, p.pk = p.parseDirectives(fdg)
	}
}

// parseInput parses an input declaration
func (p *parser) parseInput(item lexer.Item, dg *ast.DocGroup, doc *ast.Document) {
	inGen := &ast.TypeDecl{
		Doc:    dg,
		TokPos: int64(item.Pos),
		Tok:    int64(token.INPUT),
	}
	doc.Types = append(doc.Types, inGen)

	name := p.parseIdent("parseInput")

	inSpec := &ast.TypeSpec{
		Doc:  dg,
		Name: name,
	}
	inGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: inSpec}

	inType := &ast.InputType{
		Input: int64(item.Pos),
	}
	inSpec.Type = &ast.TypeSpec_Input{Input: inType}

	item = p.next()
	if item.Typ == token.AT {
		p.pk = item
		inSpec.Directives, item = p.parseDirectives(dg)
	}

	if item.Typ != token.LBRACE {
		p.pk = item
		return
	}
	p.pk = lexer.Item{}

	inType.Fields = &ast.InputValueList{
		Opening: int64(item.Pos),
	}
	inType.Fields.List, inType.Fields.Closing = p.parseArgDefs(dg)
}

// parseDirective parses a directive declaration
func (p *parser) parseDirective(item lexer.Item, dg *ast.DocGroup, doc *ast.Document) {
	dirGen := &ast.TypeDecl{
		Doc:    dg,
		TokPos: int64(item.Pos),
		Tok:    int64(token.DIRECTIVE),
	}
	doc.Types = append(doc.Types, dirGen)

	p.expect(token.AT, "parseDirective")
	name := p.expect(token.IDENT, "parseDirective")

	dirSpec := &ast.TypeSpec{
		Doc: dg,
		Name: &ast.Ident{
			NamePos: int64(name.Pos),
			Name:    name.Val,
		},
	}
	dirGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: dirSpec}

	dirType := &ast.DirectiveType{
		Directive: int64(item.Pos),
	}
	dirSpec.Type = &ast.TypeSpec_Directive{Directive: dirType}

	item = p.next()
	if item.Typ == token.LPAREN {
		args, rpos := p.parseArgDefs(dg)

		dirType.Args = &ast.InputValueList{
			Opening: int64(item.Pos),
			List:    args,
			Closing: rpos,
		}

		item = p.next()
	}

	if item.Typ != token.ON {
		p.unexpected(item, "parseDirective")
	}
	dirType.OnPos = int64(item.Pos)

	for {
		item = p.peek()
		if item.Typ != token.IDENT {
			return
		}

		loc, valid := ast.DirectiveLocation_Loc_value[item.Val]
		if !valid {
			p.unexpected(item, "parseDirectives:InvalidDirectiveLocation")
		}

		dirType.Locs = append(dirType.Locs, &ast.DirectiveLocation{Start: int64(item.Pos), Loc: ast.DirectiveLocation_Loc(loc)})
	}
}

func (p *parser) parseExtension(item lexer.Item, cdg *ast.DocGroup, d *ast.Document) {
	extGen := &ast.TypeDecl{
		Doc:    cdg,
		TokPos: int64(item.Pos),
		Tok:    int64(token.EXTEND),
	}
	d.Types = append(d.Types, extGen)

	extSpec := &ast.TypeExtensionSpec{
		Doc: cdg,
	}
	extGen.Spec = &ast.TypeDecl_TypeExtSpec{TypeExtSpec: extSpec}

	item = p.next()
	switch item.Typ {
	case token.EOF:
		return
	case token.SCHEMA:
		p.parseSchema(item, cdg, d)
	case token.SCALAR:
		p.parseScalar(item, cdg, d)
	case token.TYPE:
		p.parseObject(item, cdg, d)
	case token.INTERFACE:
		p.parseInterface(item, cdg, d)
	case token.UNION:
		p.parseUnion(item, cdg, d)
	case token.ENUM:
		p.parseEnum(item, cdg, d)
	case token.INPUT:
		p.parseInput(item, cdg, d)
	default:
		p.unexpected(item, "parseExtension")
	}

	extSpec.Type = d.Types[len(d.Types)-1].Spec.(*ast.TypeDecl_TypeSpec).TypeSpec
	d.Types = d.Types[:len(d.Types)-1]
}
