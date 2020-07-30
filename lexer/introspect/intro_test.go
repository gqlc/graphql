package introspect

import (
  "testing"
	"strings"

	"github.com/gqlc/graphql/token"
  "github.com/gqlc/graphql/lexer"
)

func TestLex(t *testing.T) {
  testCases := []struct{
    Name string
    Src string
    Intro string
  }{
    {
      Name: "Scalar",
      Src: "scalar Test",
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
  }

  for _, testCase := range testCases {
    t.Run(testCase.Name, func(subT *testing.T) {
      dset := token.NewDocSet()
      ex := lexer.Lex(dset.AddDoc("ex", -1, len(testCase.Src)), testCase.Src)

      out := Lex(dset.AddDoc("out", -1, 500), "out", strings.NewReader(testCase.Intro))

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
  }
}
