// Package lexer implements a lexer for GraphQL IDL source text.
//
package lexer

import (
	"fmt"
	"github.com/gqlc/graphql/token"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Item represents a lexed token.
type Item struct {
	Pos  token.Pos
	Line int
	Typ  token.Token
	Val  string
}

func (i Item) String() string {
	switch {
	case i.Typ == token.EOF:
		return "EOF"
	case i.Typ == token.ERR:
		return i.Val
	case i.Typ >= token.PACKAGE:
		return fmt.Sprintf("<%s>", i.Val)
	case len(i.Val) > 10:
		return fmt.Sprintf("%.10q...", i.Val)
	}
	return fmt.Sprintf("%q", i.Val)
}

// Interface defines the simplest API any consumer of a lexer could need.
type Interface interface {
	// NextItem returns the next lexed Item
	NextItem() Item

	// Drain drains the remaining items. Used only by parser if error occurs.
	Drain()
}

type lxr struct {
	// immutable state
	doc  *token.Doc
	name string
	src  string

	// scanning state
	pos       int
	start     int
	width     int
	line      int
	startLine int
	items     chan Item
}

// Lex lexs the given src based on the the GraphQL IDL specification.
func Lex(doc *token.Doc, src string) Interface {
	l := &lxr{
		doc:   doc,
		name:  doc.Name(),
		src:   src,
		items: make(chan Item, 2),
		line:  1,
	}

	go l.run()
	return l
}

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(l *lxr) stateFn

const bom = 0xFEFF

// run runs the state machine for the lexer.
func (l *lxr) run() {
	r := l.next()
	if r == bom {
		l.ignore()
	} else {
		l.backup()
	}

	for state := lexDoc; state != nil; {
		state = state(l)
	}
	close(l.items)
}

const eof = -1

// next returns the next rune in the src.
func (l *lxr) next() rune {
	if int(l.pos) >= len(l.src) {
		l.width = 0
		return eof
	}

	r, w := utf8.DecodeRuneInString(l.src[l.pos:])
	l.width = w
	l.pos += l.width
	if r == '\n' {
		l.line++
	}
	return r
}

// peek returns but does not consume the next rune in the src.
func (l *lxr) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *lxr) backup() {
	l.pos -= l.width
	// Correct newline count.
	if l.width == 1 && l.src[l.pos] == '\n' {
		l.line--
	}
}

// TODO: Check emitted value for newline characters and subtract them from l.line
// emit passes an item back to the client.
func (l *lxr) emit(t token.Token) {
	l.items <- Item{l.doc.Pos(l.start), l.line, t, l.src[l.start:l.pos]}
	l.start = l.pos
	l.startLine = l.line
}

// ignore skips over the pending src before this point.
func (l *lxr) ignore() {
	l.start = l.pos
}

// accept consumes the next rune if it's from the valid set.
func (l *lxr) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *lxr) acceptRun(valid string) {
	for strings.ContainsRune(valid, l.next()) {
	}
	l.backup()
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lxr) errorf(format string, args ...interface{}) stateFn {
	l.items <- Item{l.doc.Pos(l.start), l.line, token.ERR, fmt.Sprintf(format, args...)}
	return nil
}

// ignoreWhiteSpace consume all whitespace
func (l *lxr) ignoreWhiteSpace() {
	for r := l.next(); r == ' ' || r == '\t' || r == '\r' || r == '\n'; r = l.next() {
	}
	l.backup()
	l.ignore()
}

// ignoreSpace consumes all ' ' and '\t'
func (l *lxr) ignoreSpace() {
	for r := l.next(); r == ' ' || r == '\t'; r = l.next() {
	}
	l.backup()
	l.ignore()
}

// NextItem returns the next item from the src.
// Called by the parser, not in the lexing goroutine.
func (l *lxr) NextItem() Item {
	return <-l.items
}

// Drain drains the output so the lexing goroutine will exit.
// Called by the parser, not in the lexing goroutine.
func (l *lxr) Drain() {
	for range l.items {
	}
}

const spaceChars = " \t\r\n"

func lexDoc(l *lxr) stateFn {
	switch r := l.next(); {
	case r == eof:
		if l.pos > l.start {
			return l.errorf("unexpected eof")
		}
		l.emit(token.EOF)
		return nil
	case r == ' ', r == '\t', r == '\r', r == '\n':
		l.ignoreWhiteSpace()
	case r == '#':
		for s := l.next(); s != '\r' && s != '\n' && s != eof; {
			s = l.next()
		}
		l.emit(token.COMMENT)
	case r == '"':
		l.backup()
		if !l.scanString() {
			return l.errorf("bad string syntax: %s", l.src[l.start:l.pos])
		}
		l.emit(token.DESCRIPTION)
	case r == '@':
		l.backup()
		return l.scanDirectives(lexDoc)
	case isAlphaNumeric(r) && !unicode.IsDigit(r):
		l.backup()
		return lexDef
	}

	return lexDoc
}

func (l *lxr) scanDirectives(parent stateFn) stateFn {
	for {
		r := l.next()
		if r != '@' {
			l.backup()
			return parent
		}
		l.emit(token.AT)

		name := l.scanIdentifier()
		if name == token.ERR {
			return l.errorf("unexpected token")
		}
		l.emit(name)

		l.ignoreSpace()

		r = l.peek()
		switch r {
		case eof:
			return parent
		case '(':
			ok := l.scanArgs()
			if !ok {
				return l.errorf("invalid arg")
			}

			l.ignoreSpace()
			r = l.peek()
		}

		if r == ',' {
			l.next()
		}
		l.ignoreSpace()
	}
}

func (l *lxr) scanArgs() bool {
	if !l.accept("(") {
		return false
	}
	l.emit(token.LPAREN)

	l.ignoreWhiteSpace()

	for {
		r := l.next()
		if r == ')' {
			l.emit(token.RPAREN)
			return true
		}

		l.backup()
		if r == eof {
			return false
		}

		name := l.scanIdentifier()
		if name == token.ERR {
			return false
		}
		l.emit(name)

		l.ignoreSpace()

		if !l.accept(":") {
			return false
		}
		l.emit(token.COLON)

		l.ignoreSpace()

		ok := l.scanValue()
		if !ok {
			return false
		}

		l.acceptRun(spaceChars)
		if l.peek() == ',' {
			l.next()
			l.acceptRun(spaceChars)
		}
		l.ignore()
	}
}

// scanValue scans a Value
func (l *lxr) scanValue() (ok bool) {
	emitter := func() {}

	switch r := l.peek(); {
	case r == '$':
		l.accept("$")
		l.emit(token.VAR)
		tok := l.scanIdentifier()
		if tok == token.ERR {
			emitter = func() { l.errorf("") }
			break
		}
		emitter = func() { l.emit(tok) }
		ok = true
	case r == '"':
		ok = l.scanString()
		if !ok {
			emitter = func() { l.errorf("") }
			break
		}
		emitter = func() { l.emit(token.STRING) }
	case isAlphaNumeric(r):
		if unicode.IsDigit(r) {
			num := l.scanNumber()
			if num == token.ERR {
				emitter = func() { l.errorf("") }
				break
			}
			emitter = func() { l.emit(num) }
			ok = true
			break
		}
		tok := l.scanIdentifier()
		if tok == token.ERR {
			emitter = func() { l.errorf("") }
			break
		}
		emitter = func() { l.emit(tok) }
		ok = true
	case r == '-':
		num := l.scanNumber()
		if num == token.ERR {
			emitter = func() { l.errorf("") }
			break
		}
		emitter = func() { l.emit(num) }
		ok = true
	case r == '[':
		ok = l.scanListLit()
		if ok {
			emitter = func() { l.emit(token.RBRACK) }
		}
	case r == '{':
		ok = l.scanObjLit()
		if ok {
			emitter = func() { l.emit(token.RBRACE) }
		}
	}

	emitter()

	return
}

func (l *lxr) scanListLit() bool {
	l.accept("[")
	l.emit(token.LBRACK)

	l.ignoreWhiteSpace()

	for {
		r := l.next()
		if r == ']' {
			return true
		}

		l.backup()
		if r == eof {
			return false
		}

		ok := l.scanValue()
		if !ok {
			return false
		}

		l.acceptRun(spaceChars)
		if l.peek() == ',' {
			l.next()
			l.acceptRun(spaceChars)
		}
		l.ignore()
	}
}

func (l *lxr) scanObjLit() bool {
	l.accept("{")
	l.emit(token.LBRACE)

	l.ignoreWhiteSpace()

	for {
		r := l.next()
		if r == '}' {
			return true
		}

		l.backup()
		if r == eof {
			return false
		}

		key := l.scanIdentifier()
		if key == token.ERR {
			return false
		}
		l.emit(key)

		l.ignoreSpace()

		if !l.accept(":") {
			return false
		}
		l.emit(token.COLON)

		l.ignoreSpace()

		ok := l.scanValue()
		if !ok {
			return false
		}

		l.acceptRun(spaceChars)
		if l.peek() == ',' {
			l.next()
			l.acceptRun(spaceChars)
		}
		l.ignore()
	}
}

func lexDef(l *lxr) stateFn {
	// Ident, Ident, (Ident), (Implements/Directives/InputArguments), (on={), (Members/FieldList/InputArguments)
	declIdent := l.scanIdentifier()
	if !declIdent.IsKeyword() {
		return l.errorf("invalid type declaration")
	}
	l.emit(declIdent)

	l.ignoreSpace()

	if declIdent == token.EXTEND {
		if l.accept("@") {
			l.emit(token.AT)
		}

		declIdent = l.scanIdentifier()
		if !declIdent.IsKeyword() || declIdent == token.DIRECTIVE {
			return l.errorf("invalid type extension")
		}
		l.emit(declIdent)

		l.ignoreSpace()
	}

	var id token.Token
	r := l.peek()
	switch {
	case r == '@' && declIdent == token.DIRECTIVE:
		l.next()
		l.emit(token.AT)
		id = l.scanIdentifier()
		if id == token.ERR {
			return l.errorf("malformed directive name: %s", l.src[l.start:l.pos])
		}
		l.emit(id)
	case isAlphaNumeric(r) && !unicode.IsDigit(r):
		id = l.scanIdentifier()
		if id == token.ERR {
			return l.errorf("malformed type name: %s", l.src[l.start:l.pos])
		}
		l.emit(id)
	}

	l.ignoreSpace()

	return lexDefContents
}

func lexDefContents(l *lxr) stateFn {
declHead:
	switch r := l.next(); r {
	case eof:
		l.backup()
		return lexDoc
	case '#': // Comment
		for r = l.next(); r != '\r' && r != '\n' && r != eof; {
			r = l.next()
		}
		l.emit(token.COMMENT)

		break
	case '(': // InputArguments
		l.backup()
		ok := l.scanArgDefs()
		if !ok {
			return lexDoc
		}

		l.ignoreSpace()
		goto declHead
	case 'i': // Implements
		impls := l.scanIdentifier()
		if impls != token.IMPLEMENTS {
			return l.errorf("expected 'implements' token, not: %s", impls)
		}
		l.emit(impls)

		l.ignoreWhiteSpace()

		for {
			r := l.peek()
			if r == '&' {
				l.next()
				l.ignoreSpace()
			}
			if r == eof || r == '@' || r == '{' {
				break
			}

			loc := l.scanIdentifier()
			if loc == token.ERR {
				return l.errorf("malformed interface name")
			}
			if loc.IsKeyword() {
				l.pos = l.start
				return lexDef
			}
			l.emit(token.IDENT)

			l.ignoreWhiteSpace()
		}

		l.ignoreSpace()
		goto declHead
	case 'o': // on
		on := l.scanIdentifier()
		if on != token.ON {
			return l.errorf("expected 'on' token, not: %s", on)
		}
		l.emit(token.ON)

		l.ignoreWhiteSpace()

		for {
			r := l.peek()
			if r == '|' {
				l.next()
				l.ignoreSpace()
			}
			if r == eof {
				break
			}

			loc := l.scanIdentifier()
			if loc == token.ERR {
				return l.errorf("malformed directive location")
			}
			if loc.IsKeyword() {
				l.pos = l.start
				return lexDef
			}
			l.emit(token.IDENT)

			l.ignoreWhiteSpace()
		}
	case '@': // directives
		l.backup()

		return l.scanDirectives(lexDefContents)
	case '{': // fields
		l.backup()
		return lexFieldList
	case '=': // Union
		l.emit(token.ASSIGN)

		l.ignoreWhiteSpace()

		for {
			r := l.next()
			if r == '|' {
				l.next()
				l.ignoreSpace()
			}
			if r == eof {
				break
			}

			loc := l.scanIdentifier()
			if loc == token.ERR {
				return l.errorf("malformed union member")
			}
			if loc.IsKeyword() {
				l.pos = l.start
				return lexDef
			}
			l.emit(token.IDENT)

			l.ignoreWhiteSpace()
		}
	default:
		return l.errorf("unexpected character in type decl: %s", string(r))
	}

	return lexDoc
}

// lexFieldList lexes a list of fields
func lexFieldList(l *lxr) stateFn {
	l.accept("{")
	l.emit(token.LBRACE)

	l.ignoreWhiteSpace()

	for {
		r := l.next()
		switch {
		case r == eof:
			l.backup()
			return lexDoc
		case r == '}':
			l.emit(token.RBRACE)
			return lexDoc
		case r == '#':
			for s := l.next(); s != '\r' && s != '\n' && s != eof; {
				s = l.next()
			}
			l.emit(token.COMMENT)
		case r == '"':
			l.backup()
			if !l.scanString() {
				return l.errorf("bad string syntax: %s", l.src[l.start:l.pos])
			}
			l.emit(token.DESCRIPTION)
		case isAlphaNumeric(r) && !unicode.IsDigit(r):
			name := l.scanIdentifier()
			if name == token.ERR {
				return l.errorf("malformed field name")
			}
			l.emit(name)

			if l.peek() == '(' {
				ok := l.scanArgDefs()
				if !ok {
					return l.errorf("malformed field arguments")
				}
			}

			l.ignoreSpace()

			if l.accept(":") {
				l.emit(token.COLON)
			}

			l.ignoreSpace()

			if l.accept("@") {
				l.backup()
				f := l.scanDirectives(noopStateFn)
				if f == nil {
					return nil
				}
				break
			}

			ok := l.scanType()
			if !ok {
				return l.errorf("malformed type for field")
			}

			l.ignoreSpace()

			if l.accept("=") {
				l.emit(token.ASSIGN)
				l.ignoreSpace()

				ok := l.scanValue()
				if !ok {
					return l.errorf("malformed default value")
				}

				l.ignoreSpace()
			}

			if l.accept("@") {
				l.backup()
				f := l.scanDirectives(noopStateFn)
				if f == nil {
					return nil
				}
			}
		default:
			return l.errorf("unexpected character in field list: %s", string(r))
		}

		l.acceptRun(spaceChars)
		if l.peek() == ',' {
			l.next()
			l.acceptRun(spaceChars)
		}
		l.ignore()
	}
}

func (l *lxr) scanType() bool {
	r := l.next()
	switch {
	case r == eof:
		return false
	case r == ' ', r == '\t':
		l.ignoreSpace()
		return l.scanType()
	case r == '[':
		l.emit(token.LBRACK)
		ok := l.scanType()
		if !ok {
			return false
		}

		if !l.accept("]") {
			return false
		}
		l.emit(token.RBRACK)
	case isAlphaNumeric(r) && !unicode.IsDigit(r):
		name := l.scanIdentifier()
		if name == token.ERR {
			return false
		}
		l.emit(name)
	}

	l.ignoreSpace()
	if l.accept("!") {
		l.emit(token.NOT)
	}

	return true
}

func noopStateFn(*lxr) stateFn { return nil }

func (l *lxr) scanArgDefs() bool {
	l.accept("(")
	l.emit(token.LPAREN)

	l.ignoreWhiteSpace()

	for {
		r := l.next()
		if r == ')' {
			l.emit(token.RPAREN)
			return true
		}

		l.backup()
		if r == eof {
			return false
		}

		name := l.scanIdentifier()
		if name == token.ERR {
			return false
		}
		l.emit(name)

		l.ignoreSpace()

		if !l.accept(":") {
			return false
		}
		l.emit(token.COLON)

		l.ignoreSpace()

		ok := l.scanType()
		if !ok {
			return false
		}

		l.ignoreSpace()

		if l.accept("=") {
			l.emit(token.ASSIGN)
			l.ignoreSpace()

			ok := l.scanValue()
			if !ok {
				return false
			}

			l.ignoreSpace()
		}

		if l.accept("@") {
			l.backup()
			s := l.scanDirectives(noopStateFn)
			if s == nil {
				return false
			}
		}

		l.acceptRun(spaceChars)
		if l.peek() == ',' {
			l.next()
			l.acceptRun(spaceChars)
		}
		l.ignore()
	}
}

// scanIdentifier scans an identifier and returns its token
func (l *lxr) scanIdentifier() token.Token {
	for r := l.next(); isAlphaNumeric(r); {
		r = l.next()
	}

	l.backup()
	word := string(l.src[l.start:l.pos])
	if !l.atTerminator() {
		return token.ERR
	}

	return token.Lookup(word)
}

func (l *lxr) atTerminator() bool {
	r := l.peek()
	if isSpace(r) {
		return true
	}

	switch r {
	case eof, '.', ',', ':', ')', '(', '!', ']', '}':
		return true
	}
	return false
}

// scanString scans both a block string, `"""` and a normal string `"`
func (l *lxr) scanString() bool {
	l.acceptRun("\"")
	diff := l.pos - l.start
	if diff != 1 && diff != 3 {
		return false
	}

	for r := l.next(); r != '"' && r != eof; {
		if r == eof {
			return false
		}
		r = l.next()
	}
	l.backup()
	p := l.pos
	l.acceptRun("\"")

	newDiff := l.pos - p

	if newDiff != diff {
		return false
	}
	return true
}

// scanNumber scans both an int and a float as defined by the GraphQL spec.
func (l *lxr) scanNumber() token.Token {
	l.accept("-")
	l.acceptRun("0123456789")

	if !l.accept(".") && !l.accept("eE") {
		return token.INT
	}

	l.acceptRun("0123456789")
	l.accept("eE")
	l.accept("+-")
	l.acceptRun("0123456789")

	return token.FLOAT
}

// isAlphaNumeric reports whether r is an alphabetic, digit, or underscore.
func isAlphaNumeric(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}
