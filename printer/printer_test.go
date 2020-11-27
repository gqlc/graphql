package printer

import (
	"strings"
	"testing"

	"github.com/gqlc/graphql/parser"
	"github.com/gqlc/graphql/token"
)

func TestFprint_Document(t *testing.T) {
	docStr := `# Top Comment

@import(paths: ["hello.gql"])

scalar String`

	dset := token.NewDocSet()
	doc, err := parser.ParseDoc(dset, "test", strings.NewReader(docStr), 0)
	if err != nil {
		t.Error(err)
		return
	}

	var b strings.Builder
	err = Fprint(&b, dset, doc.Types)
	if err != nil {
		t.Error(err)
		return
	}

	if b.String() != docStr {
		t.Logf("\nexpected: %s\ngot: %s", docStr, b.String())
		t.Fail()
		return
	}
}

func TestFprint_TypeDecls(t *testing.T) {
	testCases := []struct {
		Name string
		Src  string
	}{
		{
			Name: "Scalar",
			Src:  "scalar String",
		},
		{
			Name: "Multiple Scalar",
			Src: `scalar String

scalar Int`,
		},
		{
			Name: "Union",
			Src:  "union Test = A | B | C",
		},
		{
			Name: "Directive",
			Src:  "directive @test on FIELD",
		},
		{
			Name: "Object",
			Src: `type Test implements A & B & C {
	a: A
	b: B
}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			dset := token.NewDocSet()
			doc, err := parser.ParseDoc(dset, testCase.Name, strings.NewReader(testCase.Src), 0)
			if err != nil {
				subT.Error(err)
				return
			}

			var b strings.Builder
			err = Fprint(&b, dset, doc.Types)
			if err != nil {
				subT.Error(err)
				return
			}

			if b.String() != testCase.Src {
				subT.Logf("\nexpected: %s\ngot: %s", testCase.Src, b.String())
				subT.Fail()
				return
			}
		})
	}
}
