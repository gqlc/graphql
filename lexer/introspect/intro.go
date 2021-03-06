package introspect

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/gqlc/graphql/lexer"
	"github.com/gqlc/graphql/token"
)

// Lex tokenize the results of an introspection query.
func Lex(doc *token.Doc, src io.Reader) lexer.Interface {
	s := &introScanner{
		dec:    json.NewDecoder(src),
		doc:    doc,
		items:  make(chan lexer.Item, 2),
		buf:    make(itemBuf, 0, 12),
		tmpBuf: make(itemBuf, 0, 12),
		pos:    -2,
		line:   -1,
	}

	go s.run()

	return s
}

// itemBuf is a priority queue for ordering lexer.Items after
// producing them from JSON obj keys that are not gauranteed
// to be in order.
//
type itemBuf []struct {
	priority int
	item     lexer.Item
}

func (b *itemBuf) insert(priority int, item lexer.Item) {
	*b = append(*b, struct {
		priority int
		item     lexer.Item
	}{priority, item})

	// sort on insert since JSON keys should be sorted based on query
	// thus meaning this should only be one comparision on best average.
	b.sort()
}

func (b *itemBuf) sort() {
	if len(*b) == 1 {
		return
	}

	for i := len(*b) - 1; i > 0; i-- {
		j := i
		for j > 0 {
			j--
			if (*b)[i].priority >= (*b)[j].priority {
				break
			}

			k := (*b)[j]
			(*b)[j] = (*b)[i]
			(*b)[i] = k
		}
	}
}

type introScanner struct {
	dec *json.Decoder

	name string
	doc  *token.Doc

	pos   int
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
	buf, tmpBuf itemBuf
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
		Pos:  s.doc.Pos(s.pos),
		Line: s.line,
		Typ:  t,
		Val:  val,
	}
	s.pos += len(val)
}

func (s *introScanner) emitItem(item lexer.Item) {
	s.items <- item
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
		s.emit(token.EOF, "")
		return nil
	default:
		panic("unexpected token")
	}
}

func scanDirectives(s *introScanner) stateFn {
	s.expect(json.Delim('['), "directives opening")

	for {
		tok := s.next()
		if tok == json.Delim(']') {
			return scanDoc
		}
		s.pos += 2

		if tok != json.Delim('{') {
			s.unexpected(tok, "directive opening")
		}
		s.line += 2

		s.emit(token.DIRECTIVE, "directive")
		s.pos += 1

		s.tokenizeDirectiveDecl(&s.tmpBuf)

		s.emitBuf()
	}
}

func scanTypes(s *introScanner) stateFn {
	s.expect(json.Delim('['), "types opening")

	for {
		tok := s.next()
		if tok == json.Delim(']') {
			return scanDoc
		}
		s.pos += 2

		if tok != json.Delim('{') {
			s.unexpected(tok, "type opening")
		}
		s.line += 2

		s.tokenizeTypeDecl(&s.tmpBuf)

		s.emitBuf()
	}
}

func (s *introScanner) emitBuf() {
	inArgs := false
	inList := false

	for i, itemP := range s.buf {
		item := itemP.item

		item.Pos = token.Pos(s.pos) + 1
		s.pos += len(item.Val)

		if i == len(s.buf)-1 {
			s.emitItem(item)
			break
		}

		switch {
		case item.Typ == token.AT:
		case item.Typ == token.ON:
			s.pos += 1
		case inList && item.Typ == token.RBRACK:
			item.Pos -= 1
			inList = !inList
		case item.Typ == token.LBRACK:
			inList = !inList
		case item.Typ == token.NOT:
			item.Pos -= 1
		case item.Typ == token.LPAREN:
			inArgs = !inArgs
			s.pos -= 1
			item.Pos -= 1
		case item.Typ == token.RPAREN:
			inArgs = !inArgs
			item.Pos -= 1
			if s.buf[i+1].item.Typ == token.COLON {
				s.pos -= 1
			}
		case inList && item.Typ == token.COMMA:
			s.pos += 1
			continue
		case item.Typ == token.COMMA:
			continue
		case !inArgs && item.Typ == token.LBRACE:
			s.pos += 2
		case inArgs && item.Typ == token.LBRACE:
		case inArgs && item.Typ == token.RBRACE:
			item.Pos -= 1
		default:
			if s.buf[i+1].item.Typ != token.COLON {
				s.pos += 1
			}
		}

		s.emitItem(item)
	}

	s.line = s.buf[len(s.buf)-1].item.Line
	s.buf = s.buf[:0]
}

func (s *introScanner) tokenizeDirectiveDecl(buf *itemBuf) {
	// Priorities:
	// 0 - description
	// 1 - @,name
	// 2 - args
	// 3 - repeatable
	// 4 - on, locations

	for {
		tok := s.next()
		if tok == json.Delim('}') {
			return
		}

		switch tok {
		case "description":
			tok = s.next()
			if tok == nil {
				break
			}

			descr, ok := tok.(string)
			if !ok {
				s.unexpected(tok, "description must be a string")
			}

			s.buf.insert(0, lexer.Item{Typ: token.DESCRIPTION, Val: descr, Line: s.line})
		case "name":
			tok = s.next()
			if tok == nil {
				break
			}

			name, ok := tok.(string)
			if !ok {
				s.unexpected(tok, "name must be a string")
			}

			s.buf.insert(1, lexer.Item{Typ: token.AT, Val: "@", Line: s.line})
			s.buf.insert(1, lexer.Item{Typ: token.IDENT, Val: name, Line: s.line})
		case "locations":
			tok = s.next()
			if tok == nil {
				break
			}
			if tok != json.Delim('[') {
				s.unexpected(tok, "locations opening")
			}

			s.buf.insert(4, lexer.Item{Typ: token.ON, Val: "on", Line: s.line})

			s.tokenizeLocations(&s.buf, 4)
			s.buf = s.buf[:len(s.buf)-1]
		case "args":
			tok = s.next()
			if tok == nil {
				break
			}
			if tok != json.Delim('[') {
				s.unexpected(tok, "args opening")
			}

			s.tokenizeObjList(buf, "args closing", s.tokenizeInputValue)

			if len(*buf) == 0 {
				break
			}

			s.buf.insert(2, lexer.Item{Typ: token.LPAREN, Val: "(", Line: s.line})
			*buf = (*buf)[:len(*buf)-1]
			for _, i := range *buf {
				i.item.Line = s.line
				s.buf.insert(2, i.item)
			}
			s.buf.insert(2, lexer.Item{Typ: token.RPAREN, Val: ")", Line: s.line})

			*buf = (*buf)[:0]
		case "isRepeatable":
			tok = s.next()
			if tok == nil {
				break
			}

			b, ok := tok.(bool)
			if !ok {
				s.unexpected(tok, "isRepeatable must be a bool")
			}
			if !b {
				break
			}

			s.buf.insert(3, lexer.Item{Typ: token.REPEATABLE, Val: "repeatable", Line: s.line})
		}
	}
}

func (s *introScanner) tokenizeLocations(items *itemBuf, priority int) {
	for {
		tok := s.next()
		if tok == json.Delim(']') {
			return
		}

		loc, ok := tok.(string)
		if !ok {
			s.unexpected(tok, "directive location should be a string")
		}

		items.insert(priority, lexer.Item{Typ: token.IDENT, Val: loc, Line: s.line})
		items.insert(priority, lexer.Item{Typ: token.OR, Val: "|", Line: s.line})
	}
}

func (s *introScanner) tokenizeTypeDecl(buf *itemBuf) {
	// Priorities:
	// 0 - description
	// 1 - kind
	// 2 - name
	// 3 - interfaces, =
	// 4 - fields, members, enum values

	for {
		tok := s.next()
		if tok == json.Delim('}') {
			return
		}

		switch tok {
		case "kind":
			item := s.tokenizeKind()
			item.Line = s.line
			s.buf.insert(1, item)
		case "name":
			tok = s.next()
			if tok == nil {
				break
			}

			n, ok := tok.(string)
			if !ok {
				s.unexpected(n, "name")
			}

			s.buf.insert(2, lexer.Item{Typ: token.IDENT, Val: n, Line: s.line})
		case "description":
			tok = s.next()
			if tok == nil {
				break
			}
			descr, ok := tok.(string)
			if !ok {
				s.unexpected(descr, "description")
			}

			s.buf.insert(0, lexer.Item{Typ: token.DESCRIPTION, Val: descr})
		case "fields":
			tok = s.next()
			if tok == nil {
				break
			}
			if tok != json.Delim('[') {
				s.unexpected(tok, "fields opening")
			}

			s.buf.insert(4, lexer.Item{Typ: token.LBRACE, Val: "{", Line: s.line})
			s.tokenizeObjList(buf, "fields closing", s.tokenizeField)

			*buf = (*buf)[:len(*buf)-1]
			for _, i := range *buf {
				s.buf.insert(i.priority+4, i.item)
			}
			s.buf.insert(4+len(*buf), lexer.Item{Typ: token.RBRACE, Val: "}", Line: (*buf)[len(*buf)-1].item.Line + 1})
			*buf = (*buf)[:0]
		case "interfaces":
			tok = s.next()
			if tok == nil {
				break
			}
			if tok != json.Delim('[') {
				s.unexpected(tok, "interfaces opening")
			}

			s.buf.insert(3, lexer.Item{Typ: token.IMPLEMENTS, Val: "implements", Line: s.line})

			s.tokenizeObjList(buf, "interfaces closing", s.tokenizeInterface)

			*buf = (*buf)[:len(*buf)-1]
			for _, i := range *buf {
				s.buf.insert(3, i.item)
			}
			*buf = (*buf)[:0]
		case "possibleTypes":
			tok = s.next()
			if tok == nil {
				break
			}
			if tok != json.Delim('[') {
				s.unexpected(tok, "union members opening")
			}

			s.buf.insert(3, lexer.Item{Typ: token.ASSIGN, Val: "=", Line: s.line})

			s.tokenizeObjList(buf, "union members closing", s.tokenizeMember)

			*buf = (*buf)[:len(*buf)-1]
			for _, i := range *buf {
				s.buf.insert(4, i.item)
			}
			*buf = (*buf)[:0]
		case "enumValues":
			tok = s.next()
			if tok == nil {
				break
			}
			if tok != json.Delim('[') {
				s.unexpected(tok, "enum values opening")
			}

			s.buf.insert(4, lexer.Item{Typ: token.LBRACE, Val: "{", Line: s.line})
			s.tokenizeObjList(buf, "enum values closing", s.tokenizeField)

			*buf = (*buf)[:len(*buf)-1]
			for _, i := range *buf {
				s.buf.insert(i.priority+4, i.item)
			}
			s.buf.insert(4+len(*buf), lexer.Item{Typ: token.RBRACE, Val: "}", Line: (*buf)[len(*buf)-1].item.Line + 1})
			*buf = (*buf)[:0]
		case "inputFields":
			tok = s.next()
			if tok == nil {
				break
			}
			if tok != json.Delim('[') {
				s.unexpected(tok, "input fields opening")
			}

			s.buf.insert(4, lexer.Item{Typ: token.LBRACE, Val: "{", Line: s.line})
			s.tokenizeObjList(buf, "input fields closing", s.tokenizeInputValue)

			*buf = (*buf)[:len(*buf)-1]
			for _, i := range *buf {
				s.buf.insert(i.priority+4, i.item)
			}
			s.buf.insert(4+len(*buf), lexer.Item{Typ: token.RBRACE, Val: "}", Line: (*buf)[len(*buf)-1].item.Line + 1})
			*buf = (*buf)[:0]
		case "ofType":
			tok = s.next()
			if tok != nil {
				s.unexpected(tok, "ofType should be null for a type declaration")
			}
		default:
			s.unexpected(tok, "type")
		}
	}
}

func (s *introScanner) tokenizeKind() lexer.Item {
	kind := s.next()
	switch kind {
	case "SCALAR":
		return lexer.Item{Typ: token.SCALAR, Val: "scalar", Line: s.line}
	case "OBJECT":
		return lexer.Item{Typ: token.TYPE, Val: "type", Line: s.line}
	case "INTERFACE":
		return lexer.Item{Typ: token.INTERFACE, Val: "interface", Line: s.line}
	case "UNION":
		return lexer.Item{Typ: token.UNION, Val: "union", Line: s.line}
	case "ENUM":
		return lexer.Item{Typ: token.ENUM, Val: "enum", Line: s.line}
	case "INPUT_OBJECT":
		return lexer.Item{Typ: token.INPUT, Val: "input", Line: s.line}
	default:
		s.unexpected(kind, "unknown type kind")
		return lexer.Item{}
	}
}

func (s *introScanner) tokenizeObjList(items *itemBuf, context string, f func(i int, items *itemBuf)) {
	i := 0
	for {
		tok := s.next()
		if tok == json.Delim(']') {
			return
		}
		if tok != json.Delim('{') {
			s.unexpected(tok, context)
		}

		f(i, items)
		i++
	}
}

func (s *introScanner) tokenizeInterface(i int, items *itemBuf) {
	for {
		tok := s.next()
		if tok == json.Delim('}') {
			items.insert(0, lexer.Item{Typ: token.AND, Val: "&", Line: s.line})
			return
		}

		switch tok {
		case "name":
			tok = s.next()
			if tok == nil {
				break
			}
			n, ok := tok.(string)
			if !ok {
				s.unexpected(n, "name")
			}

			items.insert(0, lexer.Item{Typ: token.IDENT, Val: n, Line: s.line})
		default:
		}
	}
}

func (s *introScanner) tokenizeMember(i int, items *itemBuf) {
	for {
		tok := s.next()
		if tok == json.Delim('}') {
			items.insert(0, lexer.Item{Typ: token.OR, Val: "|", Line: s.line})
			return
		}

		switch tok {
		case "name":
			tok = s.next()
			if tok == nil {
				break
			}
			n, ok := tok.(string)
			if !ok {
				s.unexpected(n, "name")
			}

			items.insert(0, lexer.Item{Typ: token.IDENT, Val: n, Line: s.line})
		default:
		}
	}
}

func (s *introScanner) tokenizeField(i int, items *itemBuf) {
	buf := make(itemBuf, 0, 12)

	// Priorities
	// 0 - description
	// 1 - name
	// 2 - args
	// 3 - type sig
	// 4 - deprecated directive

	iLen := len(*items)

	for {
		tok := s.next()
		if tok == json.Delim('}') {
			items.insert(iLen+3, lexer.Item{Typ: token.COMMA, Val: ",", Line: s.line + i + 1})
			return
		}

		switch tok {
		case "description":
			tok = s.next()
			if tok == nil {
				break
			}
			descr, ok := tok.(string)
			if !ok {
				s.unexpected(descr, "description")
			}

			items.insert(iLen, lexer.Item{Typ: token.DESCRIPTION, Val: descr, Line: s.line + i})
		case "name":
			tok = s.next()
			if tok == nil {
				break
			}
			n, ok := tok.(string)
			if !ok {
				s.unexpected(n, "name")
			}

			items.insert(iLen+1, lexer.Item{Typ: token.IDENT, Val: n, Line: s.line + i + 1})
		case "args":
			tok = s.next()
			if tok == nil {
				break
			}
			if tok != json.Delim('[') {
				s.unexpected(tok, "args opening")
			}

			s.tokenizeObjList(&buf, "args closing", s.tokenizeInputValue)

			if len(buf) == 0 {
				break
			}

			items.insert(iLen+2, lexer.Item{Typ: token.LPAREN, Val: "(", Line: s.line + i + 1})
			buf = buf[:len(buf)-1]
			for _, it := range buf {
				it.item.Line = s.line + i + 1
				items.insert(iLen+2, it.item)
			}
			items.insert(iLen+2, lexer.Item{Typ: token.RPAREN, Val: ")", Line: s.line + i + 1})
			items.insert(iLen+2, lexer.Item{Typ: token.COLON, Val: ":", Line: s.line + i + 1})
			buf = buf[:0]
		case "type":
			tok = s.next()
			if tok == nil {
				break
			}
			if tok != json.Delim('{') {
				s.unexpected(tok, "field type signature")
			}

			s.tokenizeTypeSig(&buf)

			for _, it := range buf {
				it.item.Line = s.line + i + 1
				items.insert(iLen+3, it.item)
			}
			buf = buf[:0]
		case "isDeprecated":
			s.next()
			// TODO
		case "deprecationReason":
			s.next()
			// TODO
		default:
			s.unexpected(tok, "field")
		}
	}
}

func (s *introScanner) tokenizeInputValue(i int, items *itemBuf) {
	buf := make(itemBuf, 0, 5)

	// Priorites
	// 0 - description
	// 1 - name
	// 2 - type signature
	// 3 - default value

	iLen := len(*items)

	for {
		tok := s.next()
		if tok == json.Delim('}') {
			items.insert(iLen+3, lexer.Item{Typ: token.COMMA, Val: ",", Line: s.line + i + 1})
			return
		}

		switch tok {
		case "name":
			tok = s.next()
			if tok == nil {
				break
			}
			n, ok := tok.(string)
			if !ok {
				s.unexpected(n, "name")
			}

			items.insert(iLen+1, lexer.Item{Typ: token.IDENT, Val: n, Line: s.line + i + 1})
			items.insert(iLen+1, lexer.Item{Typ: token.COLON, Val: ":", Line: s.line + i + 1})
		case "description":
			tok = s.next()
			if tok == nil {
				break
			}
			descr, ok := tok.(string)
			if !ok {
				s.unexpected(descr, "description")
			}

			items.insert(iLen, lexer.Item{Typ: token.DESCRIPTION, Val: descr, Line: s.line + i})
		case "type":
			tok = s.next()
			if tok == nil {
				break
			}
			if tok != json.Delim('{') {
				s.unexpected(tok, "description")
			}

			s.tokenizeTypeSig(&buf)

			for _, it := range buf {
				it.item.Line = s.line + i + 1
				items.insert(iLen+2, it.item)
			}
			buf = buf[:0]
		case "defaultValue":
			tok = s.next()
			if tok == nil {
				break
			}
			defaultVal, ok := tok.(string)
			if !ok {
				s.unexpected(defaultVal, "description")
			}

			items.insert(iLen+3, lexer.Item{Typ: token.ASSIGN, Val: "=", Line: s.line + i + 1})

			s.tokenizeValue(items, iLen+3, s.line+i+1, defaultVal)
		default:
			s.unexpected(tok, "input value")
		}
	}
}

type signature uint

const (
	unknown signature = iota
	ident
	list
	nonNull
)

func (s *introScanner) tokenizeTypeSig(items *itemBuf) {
	signa := ident

	for {
		tok := s.next()
		if tok == json.Delim('}') {
			if signa == list {
				items.insert(0, lexer.Item{Typ: token.RBRACK, Val: "]"})
			}
			if signa == nonNull {
				items.insert(0, lexer.Item{Typ: token.NOT, Val: "!"})
			}
			return
		}

		switch tok {
		case "kind":
			tok = s.next()
			if tok == nil {
				s.unexpected(tok, "type signature kind must be non-null")
			}

			sig, ok := tok.(string)
			if !ok {
				s.unexpected(tok, "type signature kind should be a string")
			}
			signa = ident

			if sig == "LIST" {
				signa = list
				*items = append(*items, struct {
					priority int
					item     lexer.Item
				}{})
				copy((*items)[1:], (*items)[0:])
				(*items)[0] = struct {
					priority int
					item     lexer.Item
				}{
					priority: 0,
					item:     lexer.Item{Typ: token.LBRACK, Val: "["},
				}
			}

			if sig == "NON_NULL" {
				signa = nonNull
			}
		case "name":
			tok = s.next()
			if tok == nil {
				break
			}

			name, ok := tok.(string)
			if !ok {
				s.unexpected(tok, "type signature type name should be a string")
			}
			if name == "" {
				break
			}

			items.insert(0, lexer.Item{Typ: token.IDENT, Val: name})
		case "ofType":
			tok = s.next()
			if tok == nil {
				break
			}
			if tok != json.Delim('{') {
				s.unexpected(tok, "type signature ofType")
			}

			s.tokenizeTypeSig(items)
		}
	}
}

func (s *introScanner) tokenizeValue(items *itemBuf, priority, line int, val string) {
	if val == "true" || val == "false" {
		items.insert(priority, lexer.Item{Typ: token.BOOL, Val: val, Line: line})
		return
	}

	if unicode.IsDigit(rune(val[0])) {
		if strings.Contains(val, ".") {
			items.insert(priority, lexer.Item{Typ: token.FLOAT, Val: val, Line: line})
			return
		}
		items.insert(priority, lexer.Item{Typ: token.INT, Val: val, Line: line})
		return
	}

	switch val[0] {
	case '"':
		items.insert(priority, lexer.Item{Typ: token.STRING, Val: val, Line: line})
	case '[':
		s.tokenizeListLit(items, priority, line, val)
	case '{':
		s.tokenizeObjLit(items, priority, line, val)
	default:
		s.errorf("invalid default value: %s", val)
	}
}

func (s *introScanner) tokenizeListLit(items *itemBuf, priority, line int, val string) {
	items.insert(priority, lexer.Item{Typ: token.LBRACK, Val: "[", Line: line})

	end := 0
	for {
		val = val[end+1:]
		if len(val) == 0 {
			items.insert(priority, lexer.Item{Typ: token.RBRACK, Val: "]", Line: line})
			return
		}

		switch val[0] {
		case '{':
			s.tokenizeObjLit(items, priority, line, val)
		case '[':
			s.tokenizeListLit(items, priority, line, val)
		default:
			end = strings.Index(val, ",")
			if end == -1 {
				end = len(val) - 1
			}

			s.tokenizeValue(items, priority, line, val[:end])
		}
	}
}

func (s *introScanner) tokenizeObjLit(items *itemBuf, priority, line int, val string) {
	items.insert(priority, lexer.Item{Typ: token.LBRACE, Val: "{", Line: line})

	end := 0
	for {
		val = val[end+1:]
		if len(val) == 0 {
			items.insert(priority, lexer.Item{Typ: token.RBRACE, Val: "}", Line: line})
			return
		}

		end = strings.Index(val, ":")
		if end == -1 {
			s.errorf("missing colon in object literal")
		}

		items.insert(priority, lexer.Item{Typ: token.IDENT, Val: val[:end], Line: line})
		items.insert(priority, lexer.Item{Typ: token.COLON, Val: ":", Line: line})
		val = val[end+1:]

		switch val[0] {
		case '{':
			s.tokenizeObjLit(items, priority, line, val)
		case '[':
			s.tokenizeListLit(items, priority, line, val)
		default:
			end = strings.Index(val, ",")
			if end == -1 {
				end = len(val) - 1
			}

			s.tokenizeValue(items, priority, line, val[:end])
		}
	}
}
