package ast

import (
	"testing"
)

var comments = []struct {
	list []string
	text string
}{
	{[]string{"\"\""}, ""},                                                          // 0
	{[]string{"\"   \""}, ""},                                                       // 1
	{[]string{"\"\"", "\"\"", "\"   \""}, ""},                                       // 2
	{[]string{"\" foo   \""}, "foo\n"},                                              // 3
	{[]string{"\"\"", "\"\"", "\" foo\""}, "foo\n"},                                 // 4
	{[]string{"\" foo  bar  \""}, "foo  bar\n"},                                     // 5
	{[]string{"\" foo\"", "\" bar\""}, "foo\nbar\n"},                                // 6
	{[]string{"\" foo\"", "\"\"", "\"\"", "\"\"", "\" bar\""}, "foo\n\nbar\n"},      // 7
	{[]string{"\" foo\"", "\"\"\" bar \"\"\""}, "foo\n bar\n"},                      // 8
	{[]string{"\"\"", "\"\"", "\"\"", "\" foo\"", "\"\"", "\"\"", "\"\""}, "foo\n"}, // 9

	{[]string{`""""""`}, ""},                                                                      // 10
	{[]string{`"""   """`}, ""},                                                                   // 11
	{[]string{`""""""`, `""""""`, `"""   """`}, ""},                                               // 12
	{[]string{`""" Foo   """`}, " Foo\n"},                                                         // 13
	{[]string{`""" Foo  Bar  """`}, " Foo  Bar\n"},                                                // 14
	{[]string{`""" Foo"""`, `""" Bar"""`}, " Foo\n Bar\n"},                                        // 15
	{[]string{`""" Foo"""`, `""""""`, `""""""`, `""""""`, `" Bar"`}, " Foo\n\nBar\n"},             // 16
	{[]string{`""" Foo"""`, "\"\"\"\n\"\"\"", `""`, "\"\"\"\n\"\"\"", `" Bar"`}, " Foo\n\nBar\n"}, // 17
	{[]string{`""" Foo"""`, `" Bar"`}, " Foo\nBar\n"},                                             // 18
	{[]string{"\"\"\" Foo\n Bar\"\"\""}, " Foo\n Bar\n"},                                          // 19
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

func BenchmarkText(b *testing.B) {
	indx := 17
	list := make([]*DocGroup_Doc, len(comments[indx].list))
	for i, s := range comments[indx].list {
		list[i] = &DocGroup_Doc{Text: s}
	}

	dg := &DocGroup{List: list}
	for i := 0; i < b.N; i++ {
		s := dg.Text()
		if s != comments[indx].text {
			b.Fail()
			return
		}
	}
}
