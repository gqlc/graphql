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
		{
			Name: "Object",
			Src: `type Test implements A & B & C {
	a(b: B): [A!]!
}`,
			Intro: `
      {
			  "__schema": {
			    "directives": [],
			    "types": [
			      {
			        "kind": "OBJECT",
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
										"kind": "NON_NULL",
										"name": null,
										"ofType": {
											"kind": "LIST",
											"name": null,
											"ofType": {
												"kind": "NON_NULL",
												"name": null,
												"ofType": {
													"kind": "OBJECT",
													"name": "A",
													"ofType": null
												}
											}
										}
									},
									"isDeprecated": false,
									"deprecationReason": null
								}
							],
			        "interfaces": [
								{
									"name": "A"
								},
								{
									"name": "B"
								},
								{
									"name": "C"
								}
							],
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
			Name: "Object With Complex fields",
			Src: `type Test implements A & B & C {
	a(b: B!): [A!]!
	b(c: [C], d: D): E
}`,
			Intro: `
      {
			  "__schema": {
			    "directives": [],
			    "types": [
			      {
			        "kind": "OBJECT",
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
												"kind": "NON_NULL",
												"name": null,
												"ofType": {
													"kind": "OBJECT",
													"name": "B",
													"ofType": null
												}
											},
											"defaultValue": null
										}
									],
									"type": {
										"kind": "NON_NULL",
										"name": null,
										"ofType": {
											"kind": "LIST",
											"name": null,
											"ofType": {
												"kind": "NON_NULL",
												"name": null,
												"ofType": {
													"kind": "OBJECT",
													"name": "A",
													"ofType": null
												}
											}
										}
									},
									"isDeprecated": false,
									"deprecationReason": null
								},
								{
									"name": "b",
									"description": null,
									"args": [
										{
											"name": "c",
											"description": null,
											"type": {
												"kind": "LIST",
												"name": null,
												"ofType": {
													"kind": "OBJECT",
													"name": "C",
													"ofType": null
												}
											},
											"defaultValue": null
										},
										{
											"name": "d",
											"description": null,
											"type": {
												"kind": "OBJECT",
												"name": "D",
												"ofType": null
											},
											"defaultValue": null
										}
									],
									"type": {
										"kind": "OBJECT",
										"name": "E",
										"ofType": null
									},
									"isDeprecated": false,
									"deprecationReason": null
								}
							],
			        "interfaces": [
								{
									"name": "A"
								},
								{
									"name": "B"
								},
								{
									"name": "C"
								}
							],
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
			Name: "Union",
			Src:  "union Test = A | B | C",
			Intro: `
      {
			  "__schema": {
			    "directives": [],
			    "types": [
			      {
			        "kind": "UNION",
			        "name": "Test",
			        "description": null,
			        "fields": null,
			        "interfaces": null,
			        "possibleTypes": [
								{
									"name": "A"
								},
								{
									"name": "B"
								},
								{
									"name": "C"
								}
							],
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
			Name: "Enum",
			Src: `enum Test {
	A
	B
	C
}`,
			Intro: `
      {
			  "__schema": {
			    "directives": [],
			    "types": [
			      {
			        "kind": "ENUM",
			        "name": "Test",
			        "description": null,
			        "fields": null,
			        "interfaces": null,
			        "possibleTypes": null,
			        "enumValues": [
								{
									"description": null,
									"name": "A",
									"isDeprecated": null,
									"deprecationReason": null
								},
								{
									"description": null,
									"name": "B",
									"isDeprecated": null,
									"deprecationReason": null
								},
								{
									"description": null,
									"name": "C",
									"isDeprecated": null,
									"deprecationReason": null
								}
							],
			        "inputFields": null,
			        "ofType": null
			      }
			    ]
			  }
			}
      `,
		},
		{
			Name: "Input Object",
			Src: `input Test {
	a: [A!]!
	b: String = "hello"
	c: Int! = 1
}`,
			Intro: `
      {
			  "__schema": {
			    "directives": [],
			    "types": [
			      {
			        "kind": "INPUT_OBJECT",
			        "name": "Test",
			        "description": null,
			        "fields": null,
			        "interfaces": null,
			        "possibleTypes": null,
			        "enumValues": null,
			        "inputFields": [
								{
									"name": "a",
									"description": null,
									"type": {
										"kind": "NON_NULL",
										"name": null,
										"ofType": {
											"kind": "LIST",
											"name": null,
											"ofType": {
												"kind": "NON_NULL",
												"name": null,
												"ofType": {
													"kind": "OBJECT",
													"name": "A",
													"ofType": null
												}
											}
										}
									},
									"defaultValue": null
								},
								{
									"name": "b",
									"description": null,
									"type": {
										"kind": "SCALAR",
										"name": "String",
										"ofType": null
									},
									"defaultValue": "\"hello\""
								},
								{
									"name": "c",
									"description": null,
									"type": {
										"kind": "NON_NULL",
										"name": null,
										"ofType": {
											"kind": "SCALAR",
											"name": "Int",
											"ofType": null
										}
									},
									"defaultValue": "1"
								}
							],
			        "ofType": null
			      }
			    ]
			  }
			}
      `,
		},
		{
			Name: "Directive",
			Src:  "directive @test(a: Int! = 1, b: [Int] = [1,2,3], c: C = {hello: \"world\",good: \"bye\"}) on FIELD_DEFINITION | ENUM_VALUE | INPUT_FIELD_DEFINITION",
			Intro: `
			{
				"__schema": {
					"types": [],
					"directives": [
						{
							"description": null,
							"name": "test",
							"locations": [
								"FIELD_DEFINITION",
								"ENUM_VALUE",
								"INPUT_FIELD_DEFINITION"
							],
							"args": [
								{
									"name": "a",
									"description": null,
									"type": {
										"kind": "NON_NULL",
										"name": null,
										"ofType": {
											"kind": "SCALAR",
											"name": "Int",
											"ofType": null
										}
									},
									"defaultValue": "1"
								},
								{
									"name": "b",
									"description": null,
									"type": {
										"kind": "LIST",
										"name": null,
										"ofType": {
											"kind": "SCALAR",
											"name": "Int",
											"ofType": null
										}
									},
									"defaultValue": "[1,2,3]"
								},
								{
									"name": "c",
									"description": null,
									"type": {
										"kind": "OBJECT",
										"name": "C",
										"ofType": null
									},
									"defaultValue": "{hello:\"world\",good:\"bye\"}"
								}
							],
							"isRepeatable": false
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
