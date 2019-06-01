// Package ast declares the types used to represent a GraphQL IDL source.
package ast

import (
	"bytes"
	"github.com/gqlc/graphql/token"
)

// Pos returns the starting position of the argument.
func (a *Arg) Pos() token.Pos {
	return a.Name.Pos()
}

// End returns the ending position of the argument.
func (a *Arg) End() token.Pos {
	switch v := a.Value.(type) {
	case *Arg_BasicLit:
		return v.BasicLit.End()
	case *Arg_CompositeLit:
		return v.CompositeLit.End()
	}
	return token.NoPos
}

// Pos returns the starting position of the field.
func (f *Field) Pos() token.Pos {
	if f.Name != nil {
		return f.Name.Pos()
	}

	switch v := f.Type.(type) {
	case *Field_Ident:
		return v.Ident.Pos()
	case *Field_List:
		return v.List.Pos()
	case *Field_NonNull:
		return v.NonNull.Pos()
	}
	return token.NoPos
}

// End returns the ending position of the field.
func (f *Field) End() token.Pos {
	if f.Directives != nil {
		return f.Directives[0].End()
	}
	switch v := f.Type.(type) {
	case *Field_Ident:
		return v.Ident.End()
	case *Field_List:
		return v.List.End()
	case *Field_NonNull:
		return v.NonNull.End()
	}
	return token.NoPos
}

// Pos returns the starting position of the field list.
func (f *FieldList) Pos() token.Pos {
	if openPos := token.Pos(f.Opening); openPos.IsValid() {
		return openPos
	}
	// the list should not be empty in this case;
	// be conservative and guard against bad ASTs
	if len(f.List) > 0 {
		return f.List[0].Pos()
	}
	return token.NoPos
}

// End returns the ending position of the field list.
func (f *FieldList) End() token.Pos {
	if closePos := token.Pos(f.Closing); closePos.IsValid() {
		return closePos + 1
	}
	// the list should not be empty in this case;
	// be conservative and guard against bad ASTs
	if n := len(f.List); n > 0 {
		return f.List[n-1].End()
	}
	return token.NoPos
}

// NumFields returns the number of parameters or struct fields represented by a FieldList.
func (f *FieldList) NumFields() (n int) {
	if f != nil {
		n = len(f.List)
	}
	return
}

func (a *InputValue) Pos() token.Pos {
	if a.Name != nil {
		return a.Name.Pos()
	}

	switch v := a.Type.(type) {
	case *InputValue_Ident:
		return v.Ident.Pos()
	case *InputValue_List:
		return v.List.Pos()
	case *InputValue_NonNull:
		return v.NonNull.Pos()
	}
	return token.NoPos
}

// End returns the ending position of the field.
func (a *InputValue) End() token.Pos {
	if a.Directives != nil {
		return a.Directives[0].End()
	}
	switch v := a.Type.(type) {
	case *InputValue_Ident:
		return v.Ident.End()
	case *InputValue_List:
		return v.List.End()
	case *InputValue_NonNull:
		return v.NonNull.End()
	}
	return token.NoPos
}

// Pos returns the starting position of the field list.
func (a *InputValueList) Pos() token.Pos {
	if openPos := token.Pos(a.Opening); openPos.IsValid() {
		return openPos
	}
	// the list should not be empty in this case;
	// be conservative and guard against bad ASTs
	if len(a.List) > 0 {
		return a.List[0].Pos()
	}
	return token.NoPos
}

// End returns the ending position of the field list.
func (a *InputValueList) End() token.Pos {
	if closePos := token.Pos(a.Closing); closePos.IsValid() {
		return closePos + 1
	}
	// the list should not be empty in this case;
	// be conservative and guard against bad ASTs
	if n := len(a.List); n > 0 {
		return a.List[n-1].End()
	}
	return token.NoPos
}

// NumValues returns the number of parameters or struct fields represented by a FieldList.
func (a *InputValueList) NumValues() (n int) {
	if a != nil {
		n = len(a.List)
	}
	return
}

// IsValidLoc returns whether or not a given string represents a valid directive location.
func IsValidLoc(l string) (DirectiveLocation_Loc, bool) {
	iLoc, ok := DirectiveLocation_Loc_value[l]
	return DirectiveLocation_Loc(iLoc), ok
}

func (x *Ident) Pos() token.Pos        { return token.Pos(x.NamePos) }
func (x *BasicLit) Pos() token.Pos     { return token.Pos(x.ValuePos) }
func (x *CompositeLit) Pos() token.Pos { return token.Pos(x.Opening) }
func (x *ListLit) Pos() token.Pos      { return token.NoPos }
func (x *ObjLit) Pos() token.Pos       { return token.NoPos }
func (x *List) Pos() token.Pos {
	switch v := x.Type.(type) {
	case *List_Ident:
		return v.Ident.Pos() - 1
	case *List_List:
		return v.List.Pos() - 1
	case *List_NonNull:
		return v.NonNull.Pos() - 1
	}
	return token.NoPos
}
func (x *NonNull) Pos() token.Pos {
	switch v := x.Type.(type) {
	case *NonNull_Ident:
		return v.Ident.Pos() - 1
	case *NonNull_List:
		return v.List.Pos() - 1
	}
	return token.NoPos
}
func (x *DirectiveLit) Pos() token.Pos      { return token.Pos(x.AtPos) }
func (x *DirectiveLocation) Pos() token.Pos { return token.Pos(x.Start) }
func (x *SchemaType) Pos() token.Pos        { return token.Pos(x.Schema) }
func (x *ScalarType) Pos() token.Pos        { return token.Pos(x.Scalar) }
func (x *ObjectType) Pos() token.Pos        { return token.Pos(x.Object) }
func (x *InterfaceType) Pos() token.Pos     { return token.Pos(x.Interface) }
func (x *UnionType) Pos() token.Pos         { return token.Pos(x.Union) }
func (x *EnumType) Pos() token.Pos          { return token.Pos(x.Enum) }
func (x *InputType) Pos() token.Pos         { return token.Pos(x.Input) }
func (x *DirectiveType) Pos() token.Pos     { return token.Pos(x.Directive) }

func (x *Ident) End() token.Pos        { return token.Pos(int(x.NamePos) + len(x.Name)) }
func (x *BasicLit) End() token.Pos     { return token.Pos(x.ValuePos) + token.Pos(len(x.Value)) }
func (x *CompositeLit) End() token.Pos { return token.Pos(x.Closing) }
func (x *ListLit) End() token.Pos      { return token.NoPos }
func (x *ObjLit) End() token.Pos       { return token.NoPos }
func (x *List) End() token.Pos {
	switch v := x.Type.(type) {
	case *List_Ident:
		return v.Ident.End() + 1
	case *List_List:
		return v.List.End() + 1
	case *List_NonNull:
		return v.NonNull.End() + 1
	}
	return token.NoPos
}
func (x *NonNull) End() token.Pos {
	switch v := x.Type.(type) {
	case *NonNull_Ident:
		return v.Ident.End() + 1
	case *NonNull_List:
		return v.List.End() + 1
	}
	return token.NoPos
}
func (x *DirectiveLit) End() token.Pos {
	if x.Args == nil {
		return token.Pos(x.AtPos) + token.Pos(len(x.Name))
	}
	return token.Pos(x.Args.Rparen)
}
func (x *DirectiveLocation) End() token.Pos {
	for k, v := range DirectiveLocation_Loc_value {
		if DirectiveLocation_Loc(v) == x.Loc {
			return token.Pos(x.Start) + token.Pos(len(k))
		}
	}
	return token.NoPos
}
func (x *SchemaType) End() token.Pos    { return x.RootOps.End() }
func (x *ScalarType) End() token.Pos    { return token.NoPos }
func (x *ObjectType) End() token.Pos    { return x.Fields.End() }
func (x *InterfaceType) End() token.Pos { return x.Fields.End() }
func (x *UnionType) End() token.Pos     { return x.Members[0].End() }
func (x *EnumType) End() token.Pos      { return x.Values.End() }
func (x *InputType) End() token.Pos     { return x.Fields.End() }
func (x *DirectiveType) End() token.Pos { return x.Locs[len(x.Locs)-1].End() }

// Pos and End implementations for spec nodes.

func (s *TypeSpec) Pos() token.Pos          { return s.Name.Pos() }
func (s *TypeExtensionSpec) Pos() token.Pos { return s.Type.Pos() }

func (s *TypeSpec) End() (e token.Pos) {
	switch v := s.Type.(type) {
	case *TypeSpec_Schema:
		e = v.Schema.End()
	case *TypeSpec_Scalar:
		e = v.Scalar.End()
	case *TypeSpec_Object:
		e = v.Object.End()
	case *TypeSpec_Interface:
		e = v.Interface.End()
	case *TypeSpec_Union:
		e = v.Union.End()
	case *TypeSpec_Enum:
		e = v.Enum.End()
	case *TypeSpec_Input:
		e = v.Input.End()
	case *TypeSpec_Directive:
		e = v.Directive.End()
	}
	if e == token.NoPos {
		e = s.Name.End()
	}
	return
}
func (s *TypeExtensionSpec) End() token.Pos {
	return s.Type.End()
}

// Text returns the text of the comment.
// Documentation markers (#, ", """), the first space of a line comment, and
// leading and trailing empty lines are removed. Multiple empty lines are
// reduced to one, and trailing space on lines is trimmed. Unless the result
// is empty, it is newline-terminated.
//
func (x *DocGroup) Text() string {
	if x == nil {
		return ""
	}

	var buf bytes.Buffer
	x.TextTo(&buf)
	return buf.String()
}

func (x *DocGroup) TextTo(buf *bytes.Buffer) {
	if x == nil {
		return
	}

	ext1 := true
	lLen := len(x.List) - 1
	for j, l := range x.List {
		c := l.Text

		// Remove comment markers.
		// The parser has given us exactly the comment text.
		c = trim(c)
		cLen := len(c) - 1
		if cLen == -1 && ext1 {
			continue
		}
		if ext1 {
			ext1 = false
		}
		if cLen == -1 && !ext1 {
			if j < lLen {
				if len(trim(x.List[j+1].Text)) != 0 {
					buf.WriteByte('\n')
					continue
				}
			}
			if j == lLen {
				break
			}
		}

		var l, n int
		ext := true
		for k := 0; k < len(c); k++ {
			if c[k] == '\n' && ext {
				l = k
				continue
			}

			if ext {
				ext = false
				l = k
				continue
			}

			if c[k] == '\n' && !ext {
				if c[l] == '\n' {
					l = k
					continue
				}

				if k-l > 0 {
					line := c[l:k]
					line = stripTrailingWhitespace(line)
					n, _ = buf.WriteString(line)
					l = k
					continue
				}
			}
		}
		if l < cLen {
			line := c[l:]
			line = stripTrailingWhitespace(line)
			n, _ = buf.WriteString(line)
		}

		if n > 0 {
			buf.WriteByte('\n')
		}
	}
}

func trim(c string) string {
	switch c[0] {
	case '#':
		// comment (no newline at the end)
		c = c[1:]
		// strip first space - required for Example tests
		if len(c) > 0 && c[0] == ' ' {
			c = c[1:]
		}
	case '"':
		c = c[1 : len(c)-1]

		if len(c) == 0 {
			break
		}

		if c[1] == '"' {
			c = c[2 : len(c)-2]
			break
		}

		// strip first space - required for Example tests
		if len(c) > 0 && c[0] == ' ' {
			c = c[1:]
		}
	}
	return c
}

func isWhitespace(ch byte) bool { return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' }

func stripTrailingWhitespace(s string) string {
	i := len(s)
	for i > 0 && isWhitespace(s[i-1]) {
		i--
	}
	return s[:i]
}
