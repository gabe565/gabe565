package main

import (
	"bytes"
	_ "embed"
	"html/template"
	"os"

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

type Section struct {
	Name  string `yaml:"name"`
	Links []Link `yaml:"links"`
}

type Link struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
	Emoji       string `yaml:"emoji"`
}

func main() {
	var sections []Section
	if err := yaml.Unmarshal(configFile, &sections); err != nil {
		panic(err)
	}

	tmpl, err := template.New("").Parse(templateFile)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	buf.Grow(len(templateFile))
	if err := tmpl.Execute(&buf, sections); err != nil {
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
