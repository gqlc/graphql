// Package token defines constants representing the lexical tokens of
// the GraphQL IDL and basic operations on tokens (printing, predicates).
//
package token

var keywords = map[string]Token{
	"schema":     SCHEMA,
	"scalar":     SCALAR,
	"type":       TYPE,
	"interface":  INTERFACE,
	"union":      UNION,
	"enum":       ENUM,
	"input":      INPUT,
	"directive":  DIRECTIVE,
	"extend":     EXTEND,
	"implements": IMPLEMENTS,
	"on":         ON,
	"true":       BOOL,
	"false":      BOOL,
	"null":       NULL,
}

// Lookup returns the appropriate token for the provided string.
func Lookup(ident string) Token {
	if tok, exists := keywords[ident]; exists {
		return tok
	}
	return IDENT
}

// Predicates

// IsLiteral returns true for tokens corresponding to identifiers
// and basic type literals; it returns false otherwise.
//
func (x Token) IsLiteral() bool { return DESCRIPTION < x && x < AND }

// IsOperator returns true for tokens corresponding to operators and
// delimiters; it returns false otherwise.
//
func (x Token) IsOperator() bool { return NULL < x && x < PACKAGE }

// IsKeyword returns true for tokens corresponding to keywords;
// it returns false otherwise.
//
func (x Token) IsKeyword() bool { return COLON < x }
