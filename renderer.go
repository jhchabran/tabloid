package tabloid

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type markdownSimplifier struct{}

// Transform will replace headings of any level by headings of the highest level, effectively
// giving the appearance of having no headings at all.
//
// Ideally, bold nodes should be returned rather than headings, but this does the job for now.
func (m *markdownSimplifier) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	for n := node.FirstChild(); n != nil; n = n.NextSibling() {
		if n.Kind() == ast.KindHeading {
			heading := n.(*ast.Heading)
			heading.Level = 6
		}
	}
}

var simplifier = markdownSimplifier{}
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.NewLinkify(
			extension.WithLinkifyAllowedProtocols([][]byte{
				[]byte("http:"),
				[]byte("https:"),
			}),
		),
	),
	goldmark.WithParserOptions(
		parser.WithASTTransformers(util.PrioritizedValue{Value: &simplifier, Priority: 100}),
	),
)

func renderBody(body string) template.HTML {
	buf := bytes.NewBufferString("")
	source := []byte(body)
	err := md.Convert(source, buf)

	if err != nil {
		return template.HTML(err.Error())
	}

	return template.HTML(buf.String())
}
