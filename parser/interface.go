// Package parser implements a parser for GraphQL IDL source files.
package parser

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gqlc/graphql/ast"
	"github.com/gqlc/graphql/token"
)

// Mode represents a parsing mode.
type Mode uint

// Mode Options
const (
	ParseComments = 1 << iota // parse comments and add them to the schema
)

// ParseDir calls ParseDoc for all files with names ending in ".gql"/".graphql" in the
// directory specified by path and returns a map of document name -> *ast.Document for all
// the documents found.
//
func ParseDir(dset *token.DocSet, path string, filter func(os.FileInfo) bool, mode Mode) (docs map[string]*ast.Document, err error) {
	if filter == nil {
		filter = func(os.FileInfo) bool { return false }
	}

	docs = make(map[string]*ast.Document)
	err = filepath.Walk(path, func(p string, info os.FileInfo, e error) error {
		skip := filter(info)
		if skip && info.IsDir() {
			return filepath.SkipDir
		}

		ext := filepath.Ext(p)
		if skip || info.IsDir() || ext != ".gql" && ext != ".graphql" {
			return nil
		}

		f, err := os.Open(p)
		if err != nil {
			return err
		}

		doc, err := ParseDoc(dset, info.Name(), f, mode)
		f.Close() // TODO: Handle this error
		if err != nil {
			return err
		}

		docs[doc.Name] = doc
		return nil
	})
	return
}

// ParseDoc parses a single GraphQL Document.
func ParseDoc(dset *token.DocSet, name string, src io.Reader, mode Mode) (*ast.Document, error) {
	// Assume src isn't massive so we're gonna just read it all
	b, err := ioutil.ReadAll(src)
	if err != nil {
		return nil, err
	}

	// Create parser and doc to doc set. Then, parse doc.
	d := dset.AddDoc(name, -1, len(b))
	p := &parser{
		name:   name,
		dg:     make([]*ast.DocGroup_Doc, 0, 4),
		cdg:    make([]*ast.DocGroup_Doc, 0, 4),
		direcs: make([]*ast.DirectiveLit, 0, 4),
		dargs:  make([]*ast.Arg, 0, 5),
		fields: make([]*ast.Field, 0, 5),
		args:   make([]*ast.InputValue, 0, 5),
		fargs:  make([]*ast.InputValue, 0, 5),
	}

	return p.parse(d, b, mode)
}

// ParseDocs parses a set of GraphQL documents. Any import paths
// in a doc will be resolved against the provided doc names in the docs map.
//
func ParseDocs(dset *token.DocSet, docs map[string]io.Reader, mode Mode) ([]*ast.Document, error) {
	odocs := make([]*ast.Document, len(docs))

	i := 0
	for name, src := range docs {
		doc, err := ParseDoc(dset, name, src, mode)
		if err != nil {
			return odocs, err
		}
		odocs[i] = doc
		i++
	}
	return odocs, nil
}

// ParseIntrospection parses the results of an introspection query. The results
// in src should be JSON encoded.
//
func ParseIntrospection(dset *token.DocSet, name string, src io.Reader) (*ast.Document, error) {
	return nil, nil
}
