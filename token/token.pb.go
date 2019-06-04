// Code generated by protoc-gen-go. DO NOT EDIT.
// source: token.proto

package token

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// Token contains all the lexical tokens for the GraphQL IDL
type Token int32

const (
	Token_UNKNOWN     Token = 0
	Token_ERR         Token = 1
	Token_EOF         Token = 2
	Token_COMMENT     Token = 3
	Token_DESCRIPTION Token = 4
	Token_IDENT       Token = 5
	Token_STRING      Token = 6
	Token_INT         Token = 7
	Token_FLOAT       Token = 8
	Token_BOOL        Token = 9
	Token_NULL        Token = 10
	Token_AND         Token = 11
	Token_OR          Token = 12
	Token_NOT         Token = 13
	Token_AT          Token = 14
	Token_VAR         Token = 15
	Token_ASSIGN      Token = 16
	Token_LPAREN      Token = 17
	Token_LBRACK      Token = 18
	Token_LBRACE      Token = 19
	Token_COMMA       Token = 20
	Token_PERIOD      Token = 21
	Token_RPAREN      Token = 22
	Token_RBRACK      Token = 23
	Token_RBRACE      Token = 24
	Token_COLON       Token = 25
	Token_PACKAGE     Token = 26
	Token_SCHEMA      Token = 27
	Token_TYPE        Token = 28
	Token_SCALAR      Token = 29
	Token_ENUM        Token = 30
	Token_INTERFACE   Token = 31
	Token_IMPLEMENTS  Token = 32
	Token_UNION       Token = 33
	Token_INPUT       Token = 34
	Token_EXTEND      Token = 35
	Token_DIRECTIVE   Token = 36
	Token_ON          Token = 37
)

var Token_name = map[int32]string{
	0:  "UNKNOWN",
	1:  "ERR",
	2:  "EOF",
	3:  "COMMENT",
	4:  "DESCRIPTION",
	5:  "IDENT",
	6:  "STRING",
	7:  "INT",
	8:  "FLOAT",
	9:  "BOOL",
	10: "NULL",
	11: "AND",
	12: "OR",
	13: "NOT",
	14: "AT",
	15: "VAR",
	16: "ASSIGN",
	17: "LPAREN",
	18: "LBRACK",
	19: "LBRACE",
	20: "COMMA",
	21: "PERIOD",
	22: "RPAREN",
	23: "RBRACK",
	24: "RBRACE",
	25: "COLON",
	26: "PACKAGE",
	27: "SCHEMA",
	28: "TYPE",
	29: "SCALAR",
	30: "ENUM",
	31: "INTERFACE",
	32: "IMPLEMENTS",
	33: "UNION",
	34: "INPUT",
	35: "EXTEND",
	36: "DIRECTIVE",
	37: "ON",
}

var Token_value = map[string]int32{
	"UNKNOWN":     0,
	"ERR":         1,
	"EOF":         2,
	"COMMENT":     3,
	"DESCRIPTION": 4,
	"IDENT":       5,
	"STRING":      6,
	"INT":         7,
	"FLOAT":       8,
	"BOOL":        9,
	"NULL":        10,
	"AND":         11,
	"OR":          12,
	"NOT":         13,
	"AT":          14,
	"VAR":         15,
	"ASSIGN":      16,
	"LPAREN":      17,
	"LBRACK":      18,
	"LBRACE":      19,
	"COMMA":       20,
	"PERIOD":      21,
	"RPAREN":      22,
	"RBRACK":      23,
	"RBRACE":      24,
	"COLON":       25,
	"PACKAGE":     26,
	"SCHEMA":      27,
	"TYPE":        28,
	"SCALAR":      29,
	"ENUM":        30,
	"INTERFACE":   31,
	"IMPLEMENTS":  32,
	"UNION":       33,
	"INPUT":       34,
	"EXTEND":      35,
	"DIRECTIVE":   36,
	"ON":          37,
}

func (x Token) String() string {
	return proto.EnumName(Token_name, int32(x))
}

func (Token) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_3aff0bcd502840ab, []int{0}
}

func init() {
	proto.RegisterEnum("Token", Token_name, Token_value)
}

func init() { proto.RegisterFile("token.proto", fileDescriptor_3aff0bcd502840ab) }

var fileDescriptor_3aff0bcd502840ab = []byte{
	// 329 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x3c, 0x91, 0x49, 0x4f, 0xeb, 0x40,
	0x10, 0x84, 0xdf, 0xcb, 0xe2, 0x24, 0xe3, 0x97, 0xa4, 0xde, 0xb0, 0xef, 0xfb, 0x85, 0x03, 0x17,
	0x7e, 0xc1, 0xc4, 0xee, 0x84, 0x51, 0xc6, 0x3d, 0xd6, 0x78, 0x1c, 0xe0, 0x8a, 0xc4, 0x09, 0x89,
	0x20, 0x94, 0xbf, 0xc6, 0xff, 0x43, 0x6d, 0x4b, 0xb9, 0x7d, 0xae, 0x76, 0x55, 0xbb, 0xcb, 0x2a,
	0xdd, 0xac, 0x3f, 0xde, 0x3f, 0x1f, 0xbe, 0xbe, 0xd7, 0x9b, 0xf5, 0xfd, 0x4f, 0x57, 0xf5, 0xa3,
	0x3c, 0xeb, 0x54, 0x0d, 0x6a, 0x5e, 0xb2, 0x7f, 0x66, 0xfc, 0xd1, 0x03, 0xd5, 0xa5, 0x10, 0xf0,
	0xb7, 0x01, 0x3f, 0x47, 0x47, 0xc6, 0x99, 0x2f, 0x0a, 0xe2, 0x88, 0xae, 0x9e, 0xaa, 0x34, 0xa7,
	0x2a, 0x0b, 0xb6, 0x8c, 0xd6, 0x33, 0x7a, 0x7a, 0xa4, 0xfa, 0x36, 0x97, 0x59, 0x5f, 0x2b, 0x95,
	0x54, 0x31, 0x58, 0x5e, 0x20, 0x11, 0xb7, 0xe5, 0x88, 0x81, 0xcc, 0xe7, 0xce, 0x9b, 0x88, 0xa1,
	0x1e, 0xaa, 0xde, 0xcc, 0x7b, 0x87, 0x91, 0x10, 0xd7, 0xce, 0x41, 0xc9, 0x7b, 0x86, 0x73, 0xa4,
	0x3a, 0x51, 0x1d, 0x1f, 0xf0, 0x4f, 0x04, 0xf6, 0x11, 0x63, 0x11, 0x4c, 0xc4, 0x44, 0x84, 0x95,
	0x09, 0x98, 0x4a, 0xbc, 0xa9, 0x2a, 0xbb, 0x60, 0x40, 0xd8, 0x95, 0x26, 0x10, 0xe3, 0x7f, 0xc3,
	0xb3, 0x60, 0xb2, 0x25, 0xf4, 0x96, 0x09, 0x3b, 0xb2, 0x59, 0xbe, 0xdb, 0x60, 0x57, 0xe4, 0x92,
	0x82, 0xf5, 0x39, 0xf6, 0x84, 0x43, 0x6b, 0xdd, 0x6f, 0xb8, 0xb5, 0x1e, 0x6c, 0x99, 0x70, 0xd8,
	0x5a, 0x9d, 0x67, 0x1c, 0xc9, 0xf5, 0xa5, 0xc9, 0x96, 0x66, 0x41, 0x38, 0x6e, 0x2e, 0xcc, 0x9e,
	0xa8, 0x30, 0x38, 0x91, 0x1b, 0xe2, 0x6b, 0x49, 0x38, 0x6d, 0x55, 0xe3, 0x4c, 0xc0, 0x99, 0xa8,
	0xc4, 0x75, 0x81, 0x73, 0x3d, 0x56, 0x23, 0xcb, 0x91, 0xc2, 0x5c, 0x22, 0x2f, 0xf4, 0x44, 0x29,
	0x5b, 0x94, 0x8e, 0xa4, 0xc7, 0x0a, 0x97, 0xb2, 0xa2, 0x66, 0xa9, 0xf0, 0xaa, 0xa9, 0x90, 0xcb,
	0x3a, 0xe2, 0x5a, 0xa2, 0xe8, 0x25, 0x12, 0xe7, 0xb8, 0x91, 0x80, 0xdc, 0x06, 0xca, 0xa2, 0x5d,
	0x11, 0x6e, 0x9b, 0x82, 0x18, 0x77, 0x6f, 0x49, 0xf3, 0xfb, 0x1e, 0x7f, 0x03, 0x00, 0x00, 0xff,
	0xff, 0xa5, 0x4d, 0xf3, 0x4b, 0xcd, 0x01, 0x00, 0x00,
}
