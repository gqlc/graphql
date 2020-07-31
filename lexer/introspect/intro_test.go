package introspect

import (
	"strings"
	"testing"

	"github.com/gqlc/graphql/lexer"
	"github.com/gqlc/graphql/token"
)

func TestLex(t *testing.T) {
	testCases := []struct {
		Name  string
		Src   string
		Intro string
	}{
		{
			Name: "Scalar",
			Src:  "scalar Test",
			Intro: `
      {
			  "__schema": {
			    "directives": [],
			    "types": [
			      {
			        "kind": "SCALAR",
			        "name": "Test",
			        "description": null,
			        "fields": null,
			        "interfaces": null,
			        "possibleTypes": null,
			        "enumValues": null,
			        "inputFields": null,
			        "ofType": null
			      }
			    ]
			  }
			}
      `,
		},
		{
			Name: "Interface",
			Src: `interface Test {
	a(b: B): A
}`,
			Intro: `
      {
			  "__schema": {
			    "directives": [],
			    "types": [
			      {
			        "kind": "INTERFACE",
			        "name": "Test",
			        "description": null,
			        "fields": [
								{
									"name": "a",
									"description": null,
									"args": [
										{
											"name": "b",
											"description": null,
											"type": {
												"kind": "OBJECT",
												"name": "B"
											},
											"defaultValue": null
										}
									],
									"type": {
										"kind": "OBJECT",
										"name": "A"
									},
									"isDeprecated": false,
									"deprecationReason": null
								}
							],
			        "interfaces": null,
			        "possibleTypes": null,
			        "enumValues": null,
			        "inputFields": null,
			        "ofType": null
			      }
			    ]
			  }
			}
      `,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			exSet := token.NewDocSet()
			ex := lexer.Lex(exSet.AddDoc("ex", -1, len(testCase.Src)), testCase.Src)

			outSet := token.NewDocSet()
			out := Lex(outSet.AddDoc("out", -1, 500), "out", strings.NewReader(testCase.Intro))

			compare(subT, ex, out)
		})
	}
}

func compare(t *testing.T, ex, out lexer.Interface) {
	defer ex.Drain()
	defer out.Drain()

	t.Helper()

	for {
		e := ex.NextItem()
		o := out.NextItem()

		if e != o {
			t.Logf("expected: %#v, got: %#v", e, o)
			t.Fail()
			return
		}

		if e.Typ == token.Token_EOF {
			return
		}
	}
}
