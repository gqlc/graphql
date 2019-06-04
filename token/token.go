// Package token defines constants representing the lexical tokens of
// the GraphQL IDL and basic operations on tokens (printing, predicates).
//
package token

var keywords = map[string]Token{
	"schema":     Token_SCHEMA,
	"scalar":     Token_SCALAR,
	"type":       Token_TYPE,
	"interface":  Token_INTERFACE,
	"union":      Token_UNION,
	"enum":       Token_ENUM,
	"input":      Token_INPUT,
	"directive":  Token_DIRECTIVE,
	"extend":     Token_EXTEND,
	"implements": Token_IMPLEMENTS,
	"on":         Token_ON,
	"true":       Token_BOOL,
	"false":      Token_BOOL,
	"null":       Token_NULL,
}

// Lookup returns the appropriate token for the provided string.
func Lookup(ident string) Token {
	if tok, exists := keywords[ident]; exists {
		return tok
	}
	return Token_IDENT
}

// Predicates

// IsLiteral returns true for tokens corresponding to identifiers
// and basic type literals; it returns false otherwise.
//
func (x Token) IsLiteral() bool { return Token_DESCRIPTION < x && x < Token_AND }

// IsOperator returns true for tokens corresponding to operators and
// delimiters; it returns false otherwise.
//
func (x Token) IsOperator() bool { return Token_NULL < x && x < Token_PACKAGE }

// IsKeyword returns true for tokens corresponding to keywords;
// it returns false otherwise.
//
func (x Token) IsKeyword() bool { return Token_COLON < x }
