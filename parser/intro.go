package parser

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/gqlc/graphql/lexer"
	"github.com/gqlc/graphql/token"
)

func scanIntrospect(doc *token.Doc, name string, src io.Reader) lexer.Interface {
	s := &introScanner{
		dec:   json.NewDecoder(src),
		doc:   doc,
		items: make(chan lexer.Item, 2),
	}

	go s.run()

	return s
}

type introScanner struct {
	dec *json.Decoder

	name string
	doc  *token.Doc

	pos   int
	start int
	width int
	line  int
	items chan lexer.Item

	// itemBuf for buffering tokens that appear out of order in the JSON
	// e.g.
	// 	"types": [
	//		{
	//			"fields": {}, -- fields tokens need to be buffered
	// 			"name": "example" -- name needs to be emitted before the fields can
	//		}
	// 	]
	//
	itemBuf []lexer.Item
}

func (s *introScanner) NextItem() lexer.Item {
	return <-s.items
}

func (s *introScanner) Drain() {
	for range s.items {
	}
}

type stateFn func(*introScanner) stateFn

func (s *introScanner) run() {
	s.expect(json.Delim('{'), "document opening")
	s.expect("__schema", "document schema")
	s.expect(json.Delim('{'), "schema")

	for state := scanDoc; state != nil; {
		state = state(s)
	}
	close(s.items)
}

func (s *introScanner) emit(t token.Token, val string) {
	s.items <- lexer.Item{
		Pos:  s.doc.Pos(s.start),
		Line: s.line,
		Typ:  t,
		Val:  val,
	}
	s.start = s.pos
}

func (s *introScanner) emitItem(item lexer.Item) {
	s.items <- item
	s.start = s.pos
}

// json.Token never represents a struct
var eof json.Token = struct{}{}

// next gets the next JSON token
func (s *introScanner) next() json.Token {
	tok, err := s.dec.Token()
	if tok == nil && err == io.EOF {
		return eof
	}
	if err != nil {
		return err
	}
	return tok
}

func (s *introScanner) expect(tok json.Token, context string) {
	t := s.next()
	if t == tok {
		return
	}

	s.unexpected(t, context)
}

func (s *introScanner) unexpected(tok json.Token, context string) {
	s.errorf("unexpected %s in %s", tok, context)
}

// errorf formats the error and terminates processing.
func (s *introScanner) errorf(format string, args ...interface{}) {
	format = fmt.Sprintf("parser: %s:%d: %s", s.name, s.line, format)
	panic(fmt.Errorf(format, args...))
}

func scanDoc(s *introScanner) stateFn {
	tok := s.next()
	switch tok {
	case "description":
		return nil
	case "queryType":
		return nil
	case "mutationType":
		return nil
	case "subscriptionType":
		return nil
	case "directives":
		return scanDirectives
	case "types":
		return scanTypes
	case json.Delim('}'):
		s.emit(token.Token_EOF, "")
		return nil
	default:
		panic("unexpected token")
	}
}

func scanDirectives(s *introScanner) stateFn {
	s.expect(json.Delim('['), "directives opening")
	s.expect(json.Delim(']'), "directives closing")
	return scanDoc
}

func scanTypes(s *introScanner) stateFn {
	s.expect(json.Delim('['), "types opening")

	tok := s.next()
	for tok != json.Delim(']') {
		if tok != json.Delim('{') {
			s.unexpected(tok, "type opening")
		}

		tok = s.tokenizeTypeDecl()
		for _, item := range s.itemBuf {
			item.Pos = token.Pos(s.pos) + 1
			s.pos += len(item.Val) + 1
			s.emitItem(item)
		}
		if tok != json.Delim('}') {
			s.unexpected(tok, "type closing")
		}

		tok = s.next()
	}
	return scanDoc
}

func (s *introScanner) tokenizeTypeDecl() json.Token {
	buf := make([]lexer.Item, 0, 12) // "descr" ident("descr" arg: Type = "default"): Type

	description, kind, name := -1, -1, -1
	interfaces, fields := -1, -1

	tok := s.next()
	for tok != json.Delim('}') {
		switch tok {
		case "kind":
			kind = description + 1
			s.tokenizeKind(&buf)
			s.itemBuf = append(s.itemBuf[:kind], append(buf, s.itemBuf[kind:]...)...)
			buf = buf[:0]
		case "name":
			tok = s.next()
			if tok == nil {
				break
			}

			n, ok := tok.(string)
			if !ok {
				s.unexpected(n, "name")
			}

			name = description + kind + 2

			s.itemBuf = append(s.itemBuf, lexer.Item{})
			copy(s.itemBuf[name+1:], s.itemBuf[name:])
			s.itemBuf[name] = lexer.Item{Typ: token.Token_IDENT, Val: n}
		case "description":
			tok = s.next()
			if tok == nil {
				break
			}
			descr, ok := tok.(string)
			if !ok {
				s.unexpected(descr, "description")
			}

			description = 0
			s.itemBuf = append(s.itemBuf, lexer.Item{})
			copy(s.itemBuf[description+1:], s.itemBuf[description:])
			s.itemBuf[description] = lexer.Item{Typ: token.Token_DESCRIPTION, Val: descr}
		case "fields":
			tok = s.next()
			if tok == nil {
				break
			}
			if tok != json.Delim('[') {
				s.unexpected(tok, "fields")
			}

			fields = description + kind + name + interfaces + 4
			s.tokenizeFields(&buf)
			s.itemBuf = append(s.itemBuf[:fields], append(buf, s.itemBuf[fields:]...)...)
			buf = buf[:0]
		case "interfaces":
			tok = s.next()
			if tok == nil {
				break
			}
		case "possibleTypes":
			tok = s.next()
			if tok == nil {
				break
			}
		case "enumValues":
			tok = s.next()
			if tok == nil {
				break
			}
		case "inputFields":
			tok = s.next()
			if tok == nil {
				break
			}
		case "ofType":
			tok = s.next()
			if tok == nil {
				break
			}
		}
		tok = s.next()
	}
	return tok
}

func (s *introScanner) tokenizeKind(items *[]lexer.Item) {
	kind := s.next()
	switch kind {
	case "SCALAR":
		*items = append(*items, lexer.Item{Typ: token.Token_SCALAR, Val: "scalar"})
	case "OBJECT":
		*items = append(*items, lexer.Item{Typ: token.Token_TYPE, Val: "type"})
	case "INTERFACE":
		*items = append(*items, lexer.Item{Typ: token.Token_INTERFACE, Val: "interface"})
	case "UNION":
		*items = append(*items, lexer.Item{Typ: token.Token_UNION, Val: "union"})
	case "ENUM":
		*items = append(*items, lexer.Item{Typ: token.Token_ENUM, Val: "enum"})
	case "INPUT_OBJECT":
		*items = append(*items, lexer.Item{Typ: token.Token_INPUT, Val: "input"})
	default:
		s.errorf("unknown type kind: %s", kind)
	}
}

func (s *introScanner) tokenizeFields(items *[]lexer.Item) {
	for {
		tok := s.next()
		if tok == json.Delim('}') {
			return
		}

	}
}
