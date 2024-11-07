package main

import (
	"bytes"
	_ "embed"
	"html"
	"html/template"
	"os"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	startTag   = "<!-- Begin projects -->\n"
	endTag     = "\n<!-- End projects -->"
	readmePath = "README.md"
)

//go:embed projects.yaml
var configFile []byte

//go:embed projects.md.tmpl
var templateFile string

type Link struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
	Logo        *Logo  `yaml:"logo"`
	Emoji       string `yaml:"emoji"`
}

type Logo struct {
	URL    string `yaml:"url"`
	Width  int    `yaml:"width"`
	Height int    `yaml:"height"`
}

func (l Link) Icon() any {
	switch {
	case l.Logo != nil:
		var buf strings.Builder
		buf.WriteString(`<img src="` + html.EscapeString(l.Logo.URL) + `"`)
		buf.WriteString(` alt="` + html.EscapeString(l.Name) + ` icon"`)

		if l.Logo.Width == 0 && l.Logo.Height == 0 {
			l.Logo.Height = 16
		}
		if l.Logo.Width != 0 {
			buf.WriteString(` width=` + strconv.Itoa(l.Logo.Width) + `px`)
		}
		if l.Logo.Height != 0 {
			buf.WriteString(` height=` + strconv.Itoa(l.Logo.Height) + `px`)
		}

		buf.WriteByte('>')
		return template.HTML(buf.String())
	case l.Emoji != "":
		return ":" + l.Emoji + ":"
	default:
		return ""
	}
}

func main() {
	var links []Link
	if err := yaml.Unmarshal(configFile, &links); err != nil {
		panic(err)
	}

	tmpl, err := template.New("").Parse(templateFile)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	buf.Grow(len(templateFile))
	if err := tmpl.Execute(&buf, links); err != nil {
		panic(err)
	}

	readme, err := os.ReadFile(readmePath)
	if err != nil {
		panic(err)
	}

	startIdx := bytes.Index(readme, []byte(startTag))
	if startIdx == -1 {
		panic("Could not find start tag: " + startTag)
	}
	startIdx += len(startTag)

	endIdx := bytes.Index(readme, []byte(endTag))
	if endIdx == -1 {
		panic("Could not find end tag: " + endTag)
	}

	var output bytes.Buffer
	output.Grow(len(readme))
	output.Write(readme[:startIdx])
	output.Write(bytes.TrimSpace(buf.Bytes()))
	output.Write(readme[endIdx:])
	if err := os.WriteFile(readmePath, output.Bytes(), 0o644); err != nil {
		panic(err)
	}
}
