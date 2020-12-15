package tabloid

import (
	"html/template"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestRenderBody(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		given    string
		expected template.HTML
	}{
		{
			given:    "# a big title\n some text",
			expected: "<h6>a big title</h6>\n<p>some text</p>\n",
		},
		{
			given:    "## a big title\n some text",
			expected: "<h6>a big title</h6>\n<p>some text</p>\n",
		},
		{
			given:    "### a big title\n some text",
			expected: "<h6>a big title</h6>\n<p>some text</p>\n",
		},
		{
			given:    "#### a big title\n some text",
			expected: "<h6>a big title</h6>\n<p>some text</p>\n",
		},
		{
			given:    "# abc\nfoo\n## def\nbar",
			expected: "<h6>abc</h6>\n<p>foo</p>\n<h6>def</h6>\n<p>bar</p>\n",
		},
	}

	for i, test := range tests {
		c.Run("RenderOK_case_"+strconv.Itoa(i), func(c *qt.C) {
			c.Assert(renderBody(test.given), qt.DeepEquals, test.expected)
		})
	}

}
