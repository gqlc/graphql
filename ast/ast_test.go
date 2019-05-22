package ast

import "testing"

var comments = []struct {
	list []string
	text string
}{
	{[]string{"\"\""}, ""},
	{[]string{"\"   \""}, ""},
	{[]string{"\"\"", "\"\"", "\"   \""}, ""},
	{[]string{"\" foo   \""}, "foo\n"},
	{[]string{"\"\"", "\"\"", "\" foo\""}, "foo\n"},
	{[]string{"\" foo  bar  \""}, "foo  bar\n"},
	{[]string{"\" foo\"", "\" bar\""}, "foo\nbar\n"},
	{[]string{"\" foo\"", "\"\"", "\"\"", "\"\"", "\" bar\""}, "foo\n\nbar\n"},
	{[]string{"\" foo\"", "\"\"\" bar \"\"\""}, "foo\n bar\n"},
	{[]string{"\"\"", "\"\"", "\"\"", "\" foo\"", "\"\"", "\"\"", "\"\""}, "foo\n"},

	{[]string{`""""""`}, ""},
	{[]string{`"""   """`}, ""},
	{[]string{`""""""`, `""""""`, `"""   """`}, ""},
	{[]string{`""" Foo   """`}, " Foo\n"},
	{[]string{`""" Foo  Bar  """`}, " Foo  Bar\n"},
	{[]string{`""" Foo"""`, `""" Bar"""`}, " Foo\n Bar\n"},
	{[]string{`""" Foo"""`, `""""""`, `""""""`, `""""""`, `" Bar"`}, " Foo\n\nBar\n"},
	{[]string{`""" Foo"""`, "\"\"\"\n\"\"\"", `""`, "\"\"\"\n\"\"\"", `" Bar"`}, " Foo\n\nBar\n"},
	{[]string{`""" Foo"""`, `" Bar"`}, " Foo\nBar\n"},
	{[]string{"\"\"\" Foo\n Bar\"\"\""}, " Foo\n Bar\n"},
}

func TestDocText(t *testing.T) {
	for i, c := range comments {
		list := make([]*DocGroup_Doc, len(c.list))
		for i, s := range c.list {
			list[i] = &DocGroup_Doc{Text: s}
		}

		text := (&DocGroup{List: list}).Text()
		if text != c.text {
			t.Errorf("case %d: got %q; expected %q", i, text, c.text)
		}
	}
}
