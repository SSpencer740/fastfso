package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

//go:embed templates/*.html
var templateFS embed.FS

type templateSet struct {
	templates map[string]*template.Template
}

func loadTemplates() (*templateSet, error) {
	base, err := template.ParseFS(templateFS, "templates/base.html")
	if err != nil {
		return nil, fmt.Errorf("parse base template: %w", err)
	}

	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("read templates dir: %w", err)
	}

	ts := &templateSet{templates: make(map[string]*template.Template)}
	for _, e := range entries {
		if e.Name() == "base.html" {
			continue
		}
		clone, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("clone base for %s: %w", e.Name(), err)
		}
		t, err := clone.ParseFS(templateFS, "templates/"+e.Name())
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		name := strings.TrimSuffix(e.Name(), ".html")
		ts.templates[name] = t
	}
	return ts, nil
}

func (ts *templateSet) render(name string, data any) (string, string, error) {
	t, ok := ts.templates[name]
	if !ok {
		return "", "", fmt.Errorf("template %q not found", name)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("execute template %q: %w", name, err)
	}

	html := buf.String()
	text := htmlToPlainText(html)
	return html, text, nil
}

// blockElements are HTML elements that should produce line breaks in plain text.
var blockElements = map[atom.Atom]bool{
	atom.P:      true,
	atom.Div:    true,
	atom.Tr:     true,
	atom.Br:     true,
	atom.H1:     true,
	atom.H2:     true,
	atom.H3:     true,
	atom.H4:     true,
	atom.H5:     true,
	atom.H6:     true,
	atom.Li:     true,
	atom.Table:  true,
	atom.Head:   true,
	atom.Style:  true,
	atom.Script: true,
}

// skipElements are HTML elements whose text content should be omitted.
var skipElements = map[atom.Atom]bool{
	atom.Head:   true,
	atom.Style:  true,
	atom.Script: true,
}

func htmlToPlainText(s string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(s))

	var buf strings.Builder
	var skipDepth int

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return collapseLines(buf.String())

		case html.StartTagToken, html.SelfClosingTagToken:
			tn, _ := tokenizer.TagName()
			a := atom.Lookup(tn)
			if skipElements[a] {
				skipDepth++
			}
			if blockElements[a] {
				buf.WriteByte('\n')
			}

		case html.EndTagToken:
			tn, _ := tokenizer.TagName()
			a := atom.Lookup(tn)
			if skipElements[a] && skipDepth > 0 {
				skipDepth--
			}
			if blockElements[a] {
				buf.WriteByte('\n')
			}

		case html.TextToken:
			if skipDepth == 0 {
				buf.Write(tokenizer.Text())
			}
		}
	}
}

func collapseLines(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
