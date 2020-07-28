package parser

import (
	"strings"
	"testing"

	"github.com/gqlc/graphql/token"
)

func TestParseIntrospection(t *testing.T) {
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			defer func() {
				r := recover()
				if r != nil {
					rerr, ok := r.(error)
					if !ok {
						subT.Error(r)
						return
					}
					subT.Error(rerr)
				}
			}()

			ex, err := ParseDoc(token.NewDocSet(), "test", strings.NewReader(testCase.Src), 0)
			if err != nil {
				subT.Error(err)
				return
			}

			intro, err := ParseIntrospection(token.NewDocSet(), "test", strings.NewReader(testCase.Intro))
			if err != nil {
				subT.Error(err)
				return
			}

			compare(subT, intro, ex)
		})
	}
}
