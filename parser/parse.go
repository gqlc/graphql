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
func ParseDoc(dset *token.DocSet, name string, src io.Reader, mode Mode) (*ast.Document, error) {
	// Assume src isn't massive so we're gonna just read it all
	b, err := ioutil.ReadAll(src)
	if err != nil {
		return nil, err
	}

	// Create parser and doc to doc set. Then, parse doc.
	p := &parser{
		name: name,
		docs: make([]*ast.DocGroup_Doc, 0, 2),
	}
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

	docs []*ast.DocGroup_Doc
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

	var docs []*ast.DocGroup_Doc
	d = &ast.Document{
		Name: doc.Name(),
	}
	defer p.recover(&err)
	p.parseDoc(&docs, d)
	if len(docs) > 0 {
		d.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(docs))}
		copy(d.Doc.List, docs)
	}
	return d, nil
}

// addDocs slurps up documentation
func (p *parser) addDocs(pdg *[]*ast.DocGroup_Doc) (cdg []*ast.DocGroup_Doc, item lexer.Item) {
	for {
		// Get next item
		item = p.next()
		isComment := item.Typ == token.Token_COMMENT
		if !isComment && item.Typ != token.Token_DESCRIPTION {
			p.pk = lexer.Item{}

			p.docs = p.docs[:0]
			return
		}

		// Skip a comment if they're not being parsed
		if isComment && p.mode&ParseComments == 0 {
			continue
		}
		cdg = append(cdg, &ast.DocGroup_Doc{
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
		*pdg = append(*pdg, cdg...)
		cdg = cdg[:0]
	}
}

// parseDoc parses a GraphQL document
func (p *parser) parseDoc(dg *[]*ast.DocGroup_Doc, d *ast.Document) {
	// Slurp up documentation
	cdg, item := p.addDocs(dg)

	switch item.Typ {
	case token.Token_ERR:
		p.unexpected(item, "parseDoc")
	case token.Token_EOF:
		return
	case token.Token_AT:
		p.parseDirectiveLit(&cdg, item, &d.Directives)
	case token.Token_SCHEMA:
		p.parseSchema(item, &cdg, d)
		d.Schema = d.Types[len(d.Types)-1]
	case token.Token_SCALAR:
		p.parseScalar(item, &cdg, d)
	case token.Token_TYPE:
		p.parseObject(item, &cdg, d)
	case token.Token_INTERFACE:
		p.parseInterface(item, &cdg, d)
	case token.Token_UNION:
		p.parseUnion(item, &cdg, d)
	case token.Token_ENUM:
		p.parseEnum(item, &cdg, d)
	case token.Token_INPUT:
		p.parseInput(item, &cdg, d)
	case token.Token_DIRECTIVE:
		p.parseDirective(item, &cdg, d)
	case token.Token_EXTEND:
		p.parseExtension(item, &cdg, d)
	}

	p.parseDoc(dg, d)
}

// parseSchema parses a schema declaration
func (p *parser) parseSchema(item lexer.Item, dg *[]*ast.DocGroup_Doc, doc *ast.Document) {
	// Create schema general decl node
	schemaDecl := &ast.TypeDecl{
		Tok:    token.Token_SCHEMA,
		TokPos: int64(item.Pos),
	}
	doc.Types = append(doc.Types, schemaDecl)
	defer func() {
		if len(*dg) == 0 {
			return
		}
		schemaDecl.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(*dg))}
		copy(schemaDecl.Doc.List, *dg)
	}()

	// Slurp up applied directives
	dirs, nitem := p.parseDirectives(dg)

	// Create schema type spec node
	schemaSpec := &ast.TypeSpec{
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

	if nitem.Typ != token.Token_LBRACE {
		p.pk = nitem
		return
	}
	schemaTyp.RootOps.Opening = int64(nitem.Pos)

	for {
		cdg, fitem := p.addDocs(dg)
		if fitem.Typ == token.Token_RBRACE {
			schemaTyp.RootOps.Closing = int64(fitem.Pos)
			return
		}

		if fitem.Typ != token.Token_IDENT {
			p.unexpected(fitem, "parseSchema")
		}

		if fitem.Val != "query" && fitem.Val != "mutation" && fitem.Val != "subscription" {
			p.unexpected(fitem, "parseSchema")
		}

		f := &ast.Field{
			Name: &ast.Ident{
				NamePos: int64(fitem.Pos),
				Name:    fitem.Val,
			},
		}
		schemaTyp.RootOps.List = append(schemaTyp.RootOps.List, f)
		if len(cdg) > 0 {
			f.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(cdg))}
			copy(f.Doc.List, cdg)
		}

		p.expect(token.Token_COLON, "parseSchema")

		fitem = p.expect(token.Token_IDENT, "parseSchema")
		f.Type = &ast.Field_Ident{Ident: &ast.Ident{
			NamePos: int64(fitem.Pos),
			Name:    fitem.Val,
		}}
	}
}

// parseDirectives parses a list of applied directives
func (p *parser) parseDirectives(dg *[]*ast.DocGroup_Doc) (dirs []*ast.DirectiveLit, item lexer.Item) {
	for {
		item = p.next()
		if item.Typ != token.Token_AT {
			return
		}

		p.parseDirectiveLit(dg, item, &dirs)
	}
}

func (p *parser) parseDirectiveLit(dg *[]*ast.DocGroup_Doc, item lexer.Item, dirs *[]*ast.DirectiveLit) {
	dir := &ast.DirectiveLit{
		AtPos: int64(item.Pos),
	}
	*dirs = append(*dirs, dir)

	item = p.expect(token.Token_IDENT, "parseDirectives")
	dir.Name = item.Val

	item = p.next()
	if item.Typ != token.Token_LPAREN {
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
func (p *parser) parseArgs(pdg *[]*ast.DocGroup_Doc) (args []*ast.Arg, rpos token.Pos) {
	for {
		_, item := p.addDocs(pdg)
		if item.Typ == token.Token_RPAREN {
			rpos = item.Pos
			return
		}

		if item.Typ != token.Token_IDENT {
			p.unexpected(item, "parseArgs:ArgName")
		}
		a := &ast.Arg{
			Name: &ast.Ident{
				NamePos: int64(item.Pos),
				Name:    item.Val,
			},
		}
		args = append(args, a)

		p.expect(token.Token_COLON, "parseArgs:MustHaveColon")

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
	case token.Token_COMMENT:
		return p.parseValue()
	case token.Token_LBRACK:
		listLit := &ast.CompositeLit_ListLit{ListLit: &ast.ListLit{}}
		compLit := &ast.CompositeLit{
			Opening: int64(item.Pos),
			Value:   listLit,
		}
		v = compLit

		var i interface{}
		for {
			item = p.peek()
			if item.Typ == token.Token_RBRACK {
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
	case token.Token_LBRACE:
		objLit := &ast.ObjLit{}
		compLit := &ast.CompositeLit{
			Opening: int64(item.Pos),
			Value:   &ast.CompositeLit_ObjLit{ObjLit: objLit},
		}
		v = compLit

		for {
			item = p.peek()
			if item.Typ == token.Token_RBRACE {
				compLit.Closing = int64(item.Pos)
				p.pk = lexer.Item{}
				return
			}

			id := p.parseIdent("parseValue:ObjectLit")

			p.expect(token.Token_COLON, "parseValue:ObjectLit")

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
	case token.Token_STRING, token.Token_INT, token.Token_FLOAT, token.Token_IDENT, token.Token_BOOL, token.Token_NULL:
		// Construct basic literal
		v = &ast.BasicLit{
			ValuePos: int64(item.Pos),
			Value:    item.Val,
			Kind:     item.Typ,
		}
		return
	default:
		p.unexpected(item, "parseValue")
	}
	return
}

func (p *parser) parseFieldDefs(pdg *[]*ast.DocGroup_Doc) (fields []*ast.Field, rpos int64) {
	for {
		cdg, item := p.addDocs(pdg)
		if item.Typ == token.Token_RBRACE {
			rpos = int64(item.Pos)
			return
		}

		if item.Typ != token.Token_IDENT && !item.Typ.IsKeyword() {
			p.unexpected(item, "parseFieldDefs:MustHaveName")
		}
		f := &ast.Field{
			Name: &ast.Ident{
				NamePos: int64(item.Pos),
				Name:    item.Val,
			},
		}
		fields = append(fields, f)
		if len(cdg) > 0 {
			f.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(cdg))}
			copy(f.Doc.List, cdg)
		}

		item = p.next()
		if item.Typ == token.Token_LPAREN {
			f.Args = &ast.InputValueList{
				Opening: int64(item.Pos),
			}
			f.Args.List, f.Args.Closing = p.parseArgDefs(&cdg)
			item = p.next()
		}
		if item.Typ != token.Token_COLON {
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
func (p *parser) parseArgDefs(pdg *[]*ast.DocGroup_Doc) (args []*ast.InputValue, rpos int64) {
	for {
		cdg, item := p.addDocs(pdg)
		if item.Typ == token.Token_RPAREN || item.Typ == token.Token_RBRACE {
			rpos = int64(item.Pos)
			return
		}

		if item.Typ != token.Token_IDENT && !item.Typ.IsKeyword() {
			p.unexpected(item, "parseArgsDef:MustHaveName")
		}
		arg := &ast.InputValue{
			Name: &ast.Ident{
				NamePos: int64(item.Pos),
				Name:    item.Val,
			},
		}
		args = append(args, arg)
		if len(cdg) > 0 {
			arg.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(cdg))}
			copy(arg.Doc.List, cdg)
		}

		p.expect(token.Token_COLON, "parseArgDefs:MustHaveColon")

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
		if item.Typ == token.Token_ASSIGN {
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
	case token.Token_LBRACK:
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
		if item.Typ != token.Token_RBRACK {
			p.unexpected(item, "parseType:MustCloseListType")
		}

		item = p.next()
		if item.Typ != token.Token_NOT {
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
	case token.Token_IDENT:
		p.pk = item
		e = p.parseIdent("parseType")

		item = p.next()
		if item.Typ != token.Token_NOT {
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
	if item.Typ != token.Token_IDENT {
		p.pk = item
		return nil
	}
	return &ast.Ident{
		NamePos: int64(item.Pos),
		Name:    item.Val,
	}
}

// parseScalar parses a scalar declaration
func (p *parser) parseScalar(item lexer.Item, dg *[]*ast.DocGroup_Doc, doc *ast.Document) {
	scalarGen := &ast.TypeDecl{
		TokPos: int64(item.Pos),
		Tok:    token.Token_SCALAR,
	}
	doc.Types = append(doc.Types, scalarGen)

	name := p.parseIdent("parseScalar")

	scalarSpec := &ast.TypeSpec{
		Name: name,
	}
	scalarGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: scalarSpec}

	scalarSpec.Directives, p.pk = p.parseDirectives(dg)

	scalarType := &ast.ScalarType{
		Scalar: int64(item.Pos),
		Name:   scalarSpec.Name,
	}
	scalarSpec.Type = &ast.TypeSpec_Scalar{Scalar: scalarType}

	if len(*dg) > 0 {
		scalarGen.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(*dg))}
		copy(scalarGen.Doc.List, *dg)
	}
}

// parseObject parses an object declaration
func (p *parser) parseObject(item lexer.Item, dg *[]*ast.DocGroup_Doc, doc *ast.Document) {
	objGen := &ast.TypeDecl{
		TokPos: int64(item.Pos),
		Tok:    token.Token_TYPE,
	}
	doc.Types = append(doc.Types, objGen)
	defer func() {
		if len(*dg) > 0 {
			objGen.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(*dg))}
			copy(objGen.Doc.List, *dg)
		}
	}()

	name := p.parseIdent("parseObject")

	objSpec := &ast.TypeSpec{
		Name: name,
	}
	objGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: objSpec}

	objType := &ast.ObjectType{
		Object: int64(item.Pos),
	}
	objSpec.Type = &ast.TypeSpec_Object{Object: objType}

	item = p.peek()
	if item.Typ == token.Token_IMPLEMENTS {
		objType.ImplPos = int64(item.Pos)
		for {
			item = p.peek()
			if item.Typ != token.Token_IDENT && item.Typ != token.Token_AND {
				break
			}
			if item.Typ == token.Token_AND {
				continue
			}

			objType.Interfaces = append(objType.Interfaces, &ast.Ident{NamePos: int64(item.Pos), Name: item.Val})
		}
	}

	if item.Typ == token.Token_AT {
		objSpec.Directives, item = p.parseDirectives(dg)
	}

	if item.Typ != token.Token_LBRACE {
		return
	}
	p.pk = lexer.Item{}

	objType.Fields = &ast.FieldList{
		Opening: int64(item.Pos),
	}
	objType.Fields.List, objType.Fields.Closing = p.parseFieldDefs(dg)
}

// parseInterface parses an interface declaration
func (p *parser) parseInterface(item lexer.Item, dg *[]*ast.DocGroup_Doc, doc *ast.Document) {
	interfaceGen := &ast.TypeDecl{
		TokPos: int64(item.Pos),
		Tok:    token.Token_INTERFACE,
	}
	doc.Types = append(doc.Types, interfaceGen)
	defer func() {
		if len(*dg) > 0 {
			interfaceGen.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(*dg))}
			copy(interfaceGen.Doc.List, *dg)
		}
	}()

	name := p.parseIdent("parseInterface")

	interfaceSpec := &ast.TypeSpec{
		Name: name,
	}
	interfaceGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: interfaceSpec}

	interfaceType := &ast.InterfaceType{
		Interface: int64(item.Pos),
	}
	interfaceSpec.Type = &ast.TypeSpec_Interface{Interface: interfaceType}

	item = p.next()
	if item.Typ == token.Token_AT {
		p.pk = item
		interfaceSpec.Directives, item = p.parseDirectives(dg)
	}

	if item.Typ != token.Token_LBRACE {
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
func (p *parser) parseUnion(item lexer.Item, dg *[]*ast.DocGroup_Doc, doc *ast.Document) {
	uGen := &ast.TypeDecl{
		TokPos: int64(item.Pos),
		Tok:    token.Token_UNION,
	}
	doc.Types = append(doc.Types, uGen)
	defer func() {
		if len(*dg) > 0 {
			uGen.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(*dg))}
			copy(uGen.Doc.List, *dg)
		}
	}()

	name := p.parseIdent("parseUnion")

	uSpec := &ast.TypeSpec{
		Name: name,
	}
	uGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: uSpec}

	uType := &ast.UnionType{
		Union: int64(item.Pos),
	}
	uSpec.Type = &ast.TypeSpec_Union{Union: uType}

	item = p.next()
	if item.Typ == token.Token_AT {
		p.pk = item
		uSpec.Directives, item = p.parseDirectives(dg)
	}

	if item.Typ != token.Token_ASSIGN {
		p.pk = item
		return
	}

	for {
		p.pk = p.next()
		if p.pk.Typ != token.Token_IDENT && p.pk.Typ != token.Token_OR {
			return
		}
		if p.pk.Typ == token.Token_OR {
			p.pk = lexer.Item{}
			continue
		}

		mem := p.parseIdent("parseUnion:Members")
		if mem == nil {
			return
		}
		uType.Members = append(uType.Members, mem)
	}
}

// parseEnum parses an enum declaration
func (p *parser) parseEnum(item lexer.Item, dg *[]*ast.DocGroup_Doc, doc *ast.Document) {
	eGen := &ast.TypeDecl{
		TokPos: int64(item.Pos),
		Tok:    token.Token_ENUM,
	}
	doc.Types = append(doc.Types, eGen)
	defer func() {
		if len(*dg) > 0 {
			eGen.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(*dg))}
			copy(eGen.Doc.List, *dg)
		}
	}()

	name := p.parseIdent("parseEnum")

	eSpec := &ast.TypeSpec{
		Name: name,
	}
	eGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: eSpec}

	eType := &ast.EnumType{
		Enum: int64(item.Pos),
	}
	eSpec.Type = &ast.TypeSpec_Enum{Enum: eType}

	item = p.next()
	if item.Typ == token.Token_AT {
		p.pk = item
		eSpec.Directives, item = p.parseDirectives(dg)
	}

	if item.Typ != token.Token_LBRACE {
		p.pk = item
		return
	}
	p.pk = lexer.Item{}

	eType.Values = &ast.FieldList{
		Opening: int64(item.Pos),
	}
	for {
		fdg, item := p.addDocs(dg)
		if item.Typ == token.Token_RBRACE {
			eType.Values.Closing = int64(item.Pos)
			p.pk = item
			return
		}
		if item.Typ != token.Token_IDENT {
			p.unexpected(item, "parseEnum:Field")
		}

		f := &ast.Field{
			Name: &ast.Ident{
				NamePos: int64(item.Pos),
				Name:    item.Val,
			},
		}
		eType.Values.List = append(eType.Values.List, f)
		if len(fdg) > 0 {
			f.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(fdg))}
			copy(f.Doc.List, fdg)
		}

		item = p.peek()
		if item.Typ != token.Token_AT {
			continue
		}
		f.Directives, p.pk = p.parseDirectives(&fdg)
	}
}

// parseInput parses an input declaration
func (p *parser) parseInput(item lexer.Item, dg *[]*ast.DocGroup_Doc, doc *ast.Document) {
	inGen := &ast.TypeDecl{
		TokPos: int64(item.Pos),
		Tok:    token.Token_INPUT,
	}
	doc.Types = append(doc.Types, inGen)
	defer func() {
		if len(*dg) > 0 {
			inGen.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(*dg))}
			copy(inGen.Doc.List, *dg)
		}
	}()

	name := p.parseIdent("parseInput")

	inSpec := &ast.TypeSpec{
		Name: name,
	}
	inGen.Spec = &ast.TypeDecl_TypeSpec{TypeSpec: inSpec}

	inType := &ast.InputType{
		Input: int64(item.Pos),
	}
	inSpec.Type = &ast.TypeSpec_Input{Input: inType}

	item = p.next()
	if item.Typ == token.Token_AT {
		p.pk = item
		inSpec.Directives, item = p.parseDirectives(dg)
	}

	if item.Typ != token.Token_LBRACE {
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
func (p *parser) parseDirective(item lexer.Item, dg *[]*ast.DocGroup_Doc, doc *ast.Document) {
	dirGen := &ast.TypeDecl{
		TokPos: int64(item.Pos),
		Tok:    token.Token_DIRECTIVE,
	}
	doc.Types = append(doc.Types, dirGen)
	defer func() {
		if len(*dg) > 0 {
			dirGen.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(*dg))}
			copy(dirGen.Doc.List, *dg)
		}
	}()

	p.expect(token.Token_AT, "parseDirective")
	name := p.expect(token.Token_IDENT, "parseDirective")

	dirSpec := &ast.TypeSpec{
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
	if item.Typ == token.Token_LPAREN {
		args, rpos := p.parseArgDefs(dg)

		dirType.Args = &ast.InputValueList{
			Opening: int64(item.Pos),
			List:    args,
			Closing: rpos,
		}

		item = p.next()
	}

	if item.Typ != token.Token_ON {
		p.unexpected(item, "parseDirective")
	}
	dirType.OnPos = int64(item.Pos)

	for {
		item = p.peek()
		if item.Typ != token.Token_IDENT && item.Typ != token.Token_OR {
			return
		}
		if item.Typ == token.Token_OR {
			p.pk = lexer.Item{}
			continue
		}

		loc, valid := ast.DirectiveLocation_Loc_value[item.Val]
		if !valid {
			p.unexpected(item, "parseDirectives:InvalidDirectiveLocation")
		}

		dirType.Locs = append(dirType.Locs, &ast.DirectiveLocation{Start: int64(item.Pos), Loc: ast.DirectiveLocation_Loc(loc)})
	}
}

func (p *parser) parseExtension(item lexer.Item, cdg *[]*ast.DocGroup_Doc, d *ast.Document) {
	extGen := &ast.TypeDecl{
		TokPos: int64(item.Pos),
		Tok:    token.Token_EXTEND,
	}
	d.Types = append(d.Types, extGen)
	defer func() {
		if len(*cdg) > 0 {
			extGen.Doc = &ast.DocGroup{List: make([]*ast.DocGroup_Doc, len(*cdg))}
			copy(extGen.Doc.List, *cdg)
		}
	}()

	item = p.next()
	switch item.Typ {
	case token.Token_EOF:
		return
	case token.Token_SCHEMA:
		p.parseSchema(item, cdg, d)
	case token.Token_SCALAR:
		p.parseScalar(item, cdg, d)
	case token.Token_TYPE:
		p.parseObject(item, cdg, d)
	case token.Token_INTERFACE:
		p.parseInterface(item, cdg, d)
	case token.Token_UNION:
		p.parseUnion(item, cdg, d)
	case token.Token_ENUM:
		p.parseEnum(item, cdg, d)
	case token.Token_INPUT:
		p.parseInput(item, cdg, d)
	default:
		p.unexpected(item, "parseExtension")
	}

	extSpec := &ast.TypeExtensionSpec{
		TokPos: int64(item.Pos),
		Tok:    item.Typ,
	}
	extGen.Spec = &ast.TypeDecl_TypeExtSpec{TypeExtSpec: extSpec}
	extSpec.Type = d.Types[len(d.Types)-1].Spec.(*ast.TypeDecl_TypeSpec).TypeSpec
	d.Types = d.Types[:len(d.Types)-1]
}
