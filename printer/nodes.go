package printer

import (
	"github.com/gqlc/graphql/ast"
	"github.com/gqlc/graphql/token"
)

// Print as many newlines as necessary (but at least min newlines) to get to
// the current line. ws is printed before the first line break. If newSection
// is set, the first line break is printed as formfeed. Returns 0 if no line
// breaks were printed, returns 1 if there was exactly one newline printed,
// and returns a value > 1 if there was a formfeed or more than one newline
// printed.
//
// TODO(gri): linebreak may add too many lines if the next statement at "line"
//            is preceded by comments because the computation of n assumes
//            the current position before the comment and the target position
//            after the comment. Thus, after interspersing such comments, the
//            space taken up by them is not considered to reduce the number of
//            linebreaks. At the moment there is no easy way to know about
//            future (not yet interspersed) comments in this function.
//
func (p *printer) linebreak(line, min int, ws whiteSpace, newSection bool) (nbreaks int) {
	n := nlimit(line - p.pos.Line)
	if n < min {
		n = min
	}
	if n > 0 {
		p.print(ws)
		if newSection {
			p.print(formfeed)
			n--
			nbreaks = 2
		}
		nbreaks += n
		for ; n > 0; n-- {
			p.print(newline)
		}
	}
	return
}

func (p *printer) argList(args *ast.InputValueList) {}

func (p *printer) field(f *ast.Field) {
	p.print(f.Name)
	if f.Args != nil {
		p.argList(f.Args)
	}
	p.print(":", blank)
	switch x := f.Type.(type) {
	case *ast.Field_Ident:
		p.print(x.Ident)
	case *ast.Field_List:
		p.print(x.List)
	case *ast.Field_NonNull:
		p.print(x.NonNull)
	}
}

func (p *printer) fieldList(fields *ast.FieldList) {
	lbrace := token.Pos(fields.Opening)
	rbrace := token.Pos(fields.Closing)
	p.print(lbrace, "{", indent)
	l := len(fields.List)
	if l > 0 {
		p.print(formfeed)
	}
	var line int
	for i, field := range fields.List {
		if i > 0 {
			p.linebreak(p.lineFor(field.Pos()), 1, ignore, p.linesFrom(line) > 0)
		}
		p.recordLine(&line)
		p.field(field)
	}
	p.print(unindent, formfeed, rbrace, "}")
}

func (p *printer) spec(ts *ast.TypeSpec) {
	switch x := ts.Type.(type) {
	case *ast.TypeSpec_Scalar:
		return // TODO: print any directives
	case *ast.TypeSpec_Union:
		union := x.Union
		// = A | B | C ...
		args := make([]interface{}, 0, 4*len(union.Members))
		args = append(args, blank, "=", blank)
		l := len(union.Members)
		for i, m := range union.Members {
			args = append(args, m)
			if i < l-1 {
				args = append(args, blank, "|", blank)
			}
		}
		p.print(args...)
	case *ast.TypeSpec_Directive:
		direc := x.Directive
		p.print(blank, token.Token_ON, blank)
		l := len(direc.Locs)
		for i, loc := range direc.Locs {
			p.print(loc)
			if i < l-1 {
				p.print(blank, "|", blank)
			}
		}
	case *ast.TypeSpec_Object:
		p.print(blank)

		obj := x.Object
		il := len(obj.Interfaces)
		if il > 0 {
			p.print(token.Token_IMPLEMENTS, blank)
		}
		for i, inter := range obj.Interfaces {
			p.print(inter)
			if i < il-1 {
				p.print(blank, "&", blank)
			} else {
				p.print(blank)
			}
		}

		p.fieldList(obj.Fields)
	}
}

func (p *printer) decl(d *ast.TypeDecl) {
	var isExt bool
	var ts *ast.TypeSpec
	dts, ok := d.Spec.(*ast.TypeDecl_TypeSpec)
	if !ok {
		dts := d.Spec.(*ast.TypeDecl_TypeExtSpec)
		ts = dts.TypeExtSpec.Type
		isExt = true
	} else {
		ts = dts.TypeSpec
	}

	if isExt {
		p.print(token.Token_EXTEND, blank)
	}

	p.print(d.Tok, blank)
	if d.Tok == token.Token_DIRECTIVE {
		p.print("@")
	}
	p.print(ts.Name)

	p.spec(ts)
}

func (p *printer) declList(decls []*ast.TypeDecl) {
	l := len(decls)
	for i, d := range decls {
		p.decl(d)
		if i < l-1 {
			p.print(newline, newline)
		}
	}
}
