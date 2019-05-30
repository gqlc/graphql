package lexer

import (
	"fmt"
	"github.com/gqlc/graphql/token"
	"strings"
	"unicode"
	"unicode/utf8"
)

type lxr2 struct {
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
func Lex2(doc *token.Doc, src string) Interface {
	l := &lxr2{
		doc:   doc,
		name:  doc.Name(),
		src:   src,
		items: make(chan Item, 2),
		line:  1,
	}

	go l.run()
	return l
}

type stateFn2 func(l *lxr2) stateFn2

// run runs the state machine for the lexer.
func (l *lxr2) run() {
	r := l.next()
	if r == bom {
		l.ignore()
	} else {
		l.backup()
	}

	for state := lexDoc2; state != nil; {
		state = state(l)
	}
	close(l.items)
}

// next returns the next rune in the src.
func (l *lxr2) next() rune {
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
func (l *lxr2) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *lxr2) backup() {
	l.pos -= l.width
	// Correct newline count.
	if l.width == 1 && l.src[l.pos] == '\n' {
		l.line--
	}
}

// TODO: Check emitted value for newline characters and subtract them from l.line
// emit passes an item back to the client.
func (l *lxr2) emit(t token.Token) {
	l.items <- Item{t, l.doc.Pos(l.start), l.src[l.start:l.pos], l.line}
	l.start = l.pos
	l.startLine = l.line
}

// ignore skips over the pending src before this point.
func (l *lxr2) ignore() {
	l.start = l.pos
}

// accept consumes the next rune if it's from the valid set.
func (l *lxr2) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *lxr2) acceptRun(valid string) {
	for strings.ContainsRune(valid, l.next()) {
	}
	l.backup()
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lxr2) errorf(format string, args ...interface{}) stateFn2 {
	l.items <- Item{token.ERR, l.doc.Pos(l.start), fmt.Sprintf(format, args...), l.line}
	return nil
}

// NextItem returns the next item from the src.
// Called by the parser, not in the lexing goroutine.
//
func (l *lxr2) NextItem() Item {
	return <-l.items
}

// Drain drains the output so the lexing goroutine will exit.
// Called by the parser, not in the lexing goroutine.
//
func (l *lxr2) Drain() {
	for range l.items {
	}
}

func lexDoc2(l *lxr2) stateFn2 {
	switch r := l.next(); {
	case r == eof:
		if l.pos > l.start {
			return l.errorf("unexpected eof")
		}
		l.emit(token.EOF)
		return nil
	case r == ' ', r == '\t', r == '\r', r == '\n':
		l.acceptRun(spaceChars)
		l.ignore()
	case r == '#':
		for s := l.next(); s != '\r' && s != '\n' && s != eof; {
			s = l.next()
		}
		l.emit(token.COMMENT)
	case r == '"':
		l.backup()
		if !l.scanString() {
			return l.errorf("bad string syntax: %q", l.src[l.start:l.pos])
		}
		l.emit(token.DESCRIPTION)
	case r == '@':
		l.backup()
		return l.scanDirectives(lexDoc2)
	case isAlphaNumeric(r) && !unicode.IsDigit(r):
		l.backup()
		return lexDef2
	}

	return lexDoc2
}

func (l *lxr2) scanDirectives(parent stateFn2) stateFn2 {
	for {
		r := l.next()
		if r != '@' {
			return parent
		}
		l.emit(token.AT)

		name := l.scanIdentifier()
		if name == token.ERR {
			return l.errorf("unexpected token")
		}
		l.emit(name)

		l.acceptRun(" \t")
		l.ignore()

		r = l.peek()
		switch r {
		case eof:
			return parent
		case '(':
			ok := l.scanArgs()
			if !ok {
				return l.errorf("invalid arg")
			}

			l.acceptRun(" \t")
			l.ignore()
			r = l.peek()
		}

		if r == ',' {
			l.next()
			l.ignore()
			l.acceptRun(" \t")
			l.ignore()
		}
	}
}

func (l *lxr2) scanArgs() bool {
	if !l.accept("(") {
		return false
	}
	l.emit(token.LPAREN)

	l.acceptRun(spaceChars)
	l.ignore()

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

		l.acceptRun(" \t")
		l.ignore()

		if !l.accept(":") {
			return false
		}

		l.acceptRun(" \t")
		l.ignore()

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
func (l *lxr2) scanValue() (ok bool) {
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

func (l *lxr2) scanListLit() bool {
	l.accept("[")
	l.emit(token.LBRACK)

	l.acceptRun(spaceChars)
	l.ignore()

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

func (l *lxr2) scanObjLit() bool {
	l.accept("{")
	l.emit(token.LBRACE)

	l.acceptRun(spaceChars)
	l.ignore()

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

		l.acceptRun(" \t")
		l.ignore()

		if !l.accept(":") {
			return false
		}
		l.emit(token.COLON)

		l.acceptRun(" \t")
		l.ignore()

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

func lexDef2(l *lxr2) stateFn2 {
	// (Ident), Ident, (@), Ident, (Implements/Directives/InputArguments), (on={), (Members/FieldList/InputArguments)
	declIdent := l.scanIdentifier()
	if !declIdent.IsKeyword() {
		return l.errorf("invalid type declaration")
	}
	l.emit(declIdent)

	l.acceptRun(" \t")
	l.ignore()

	if l.peek() == '@' {
		l.next()
		l.emit(token.AT)
	}

	id := l.scanIdentifier()
	switch {
	case id == token.ERR:
		return l.errorf("unexpected error in identifier: %s", id)
	case id.IsKeyword():
		l.emit(id)
		l.acceptRun(" \t")
		l.ignore()

		name := l.scanIdentifier()
		if name != token.IDENT {
			return l.errorf("invalid type extension name")
		}
		l.emit(name)
	default:
		l.emit(id)
	}
	l.acceptRun(" \t")
	l.ignore()

	return lexDefContents
}

func lexDefContents(l *lxr2) stateFn2 {
declHead:
	switch r := l.next(); r {
	case eof:
		l.backup()
		return lexDoc2
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
			return lexDoc2
		}

		l.acceptRun(" \t")
		l.ignore()
		goto declHead
	case 'i': // Implements
		impls := l.scanIdentifier()
		if impls != token.IMPLEMENTS {
			return l.errorf("expected 'implements' token, not: %s", impls)
		}
		l.emit(impls)

		l.acceptRun(spaceChars)
		l.ignore()

		for {
			r := l.peek()
			if r == '&' {
				l.next()
				l.acceptRun(" \t")
				l.ignore()
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
				return lexDef2
			}
			l.emit(token.IDENT)

			l.acceptRun(spaceChars)
			l.ignore()
		}

		l.acceptRun(" \t")
		l.ignore()
		goto declHead
	case 'o': // on
		on := l.scanIdentifier()
		if on != token.ON {
			return l.errorf("expected 'on' token, not: %s", on)
		}
		l.emit(token.ON)

		l.acceptRun(spaceChars)
		l.ignore()

		for {
			r := l.peek()
			if r == '|' {
				l.next()
				l.acceptRun(" \t")
				l.ignore()
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
				return lexDef2
			}
			l.emit(token.IDENT)

			l.acceptRun(spaceChars)
			l.ignore()
		}
	case '@': // directives
		l.backup()

		return l.scanDirectives(lexDefContents)
	case '{': // fields
		l.backup()
		return lexFieldList
	case '=': // Union
		l.emit(token.ASSIGN)

		l.acceptRun(spaceChars)
		l.ignore()

		for {
			r := l.peek()
			if r == '|' {
				l.next()
				l.acceptRun(" \t")
				l.ignore()
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
				return lexDef2
			}
			l.emit(token.IDENT)

			l.acceptRun(spaceChars)
			l.ignore()
		}
	default:
		return l.errorf("unexpected character in type decl: %s", string(r))
	}

	return lexDoc2
}

// lexFieldList lexes a list of fields
func lexFieldList(l *lxr2) stateFn2 {
	if !l.accept("{") {
		return l.errorf("expected '{' to begin field list")
	}
	l.emit(token.LBRACE)

	l.acceptRun(spaceChars)
	l.ignore()

	for {
		r := l.next()
		switch {
		case r == eof:
			l.backup()
			return lexDoc2
		case r == '}':
			l.emit(token.RBRACE)
			return lexDoc2
		case r == '#':
			for s := l.next(); s != '\r' && s != '\n' && s != eof; {
				s = l.next()
			}
			l.emit(token.COMMENT)
		case r == '"':
			l.backup()
			if !l.scanString() {
				return l.errorf("bad string syntax: %q", l.src[l.start:l.pos])
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

			l.acceptRun(" \t")
			l.ignore()

			if l.accept(":") {
				l.emit(token.COLON)
			}

			l.acceptRun(" \t")
			l.ignore()

			if l.accept("@") {
				f := l.scanDirectives(noopStateFn)
				if f == nil {
					return nil
				}
				continue
			}

			ok := l.scanType()
			if !ok {
				return l.errorf("malformed type for field")
			}

			l.acceptRun(" \t")
			l.ignore()

			if l.accept("@") {
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

func (l *lxr2) scanType() bool {
	r := l.next()
	switch {
	case r == eof:
		return false
	case r == ' ', r == '\t':
		l.acceptRun(" \t")
		l.ignore()
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

	l.acceptRun(" \t")
	l.ignore()
	if l.accept("!") {
		l.emit(token.NOT)
	}

	return true
}

func noopStateFn(*lxr2) stateFn2 { return nil }

func (l *lxr2) scanArgDefs() bool {
	l.accept("(")
	l.emit(token.LPAREN)

	l.acceptRun(spaceChars)
	l.ignore()

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

		l.acceptRun(" \t")
		l.ignore()

		if !l.accept(":") {
			return false
		}
		l.emit(token.COLON)

		l.acceptRun(" \t")
		l.ignore()

		ok := l.scanType()
		if !ok {
			return false
		}

		l.acceptRun(" \t")
		l.ignore()

		if l.accept("=") {
			l.emit(token.ASSIGN)
			l.acceptRun(" \t")
			l.ignore()

			ok := l.scanValue()
			if !ok {
				return false
			}

			l.acceptRun(" \t")
			l.ignore()
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
func (l *lxr2) scanIdentifier() token.Token {
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

func (l *lxr2) atTerminator() bool {
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
func (l *lxr2) scanString() bool {
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
func (l *lxr2) scanNumber() token.Token {
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
