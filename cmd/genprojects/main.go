package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

const (
	startTag   = "<!-- Begin projects -->\n"
	endTag     = "\n<!-- End projects -->"
	readmePath = "README.md"
)

//go:embed projects.md.tmpl
var templateFile string

var (
	configFile          = "projects.yaml"
	githubToken         string
	githubCacheDuration = time.Hour
)

func init() {
	if env := os.Getenv("GH_TOKEN"); env != "" {
		githubToken = env
	} else if env := os.Getenv("GITHUB_TOKEN"); env != "" {
		githubToken = env
	}
}

type Link struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url"`
	Source      string `yaml:"source"`
	Description string `yaml:"description"`
	Logo        *Logo  `yaml:"logo"`
	Emoji       string `yaml:"emoji"`
}

var ErrUpstream = errors.New("upstream returned an error")

func (l *Link) FetchGitHubDescription() error {
	repoUrl := l.Source
	if repoUrl == "" {
		repoUrl = l.URL
	}
	if !strings.HasPrefix(repoUrl, "https://github.com/") {
		return nil
	}

	u := &url.URL{
		Scheme: "https",
		Host:   "api.github.com",
		Path:   path.Join("repos", strings.TrimPrefix(repoUrl, "https://github.com/")),
	}

	cachePath := filepath.Join(".cache", u.Path)
	if stat, err := os.Stat(cachePath); err == nil {
		if time.Since(stat.ModTime()) < githubCacheDuration {
			if b, err := os.ReadFile(cachePath); err == nil {
				l.Description = string(b)
				return nil
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	if githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+githubToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %w: %s\n%s", u.String(), ErrUpstream, resp.Status, body)
	}

	var repo map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return err
	}

	l.Description = repo["description"].(string)
	l.Description = strings.TrimSpace(l.Description)
	return os.WriteFile(cachePath, []byte(l.Description), 0o644)
}

type Logo struct {
	URL    string `yaml:"url"`
	Width  int    `yaml:"width"`
	Height int    `yaml:"height"`
}

func (l *Link) Icon() any {
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
	fs := pflag.NewFlagSet("genprojects", pflag.ExitOnError)
	fs.StringVarP(&configFile, "config", "c", configFile, "path to config file")
	fs.StringVar(&githubToken, "github-token", githubToken, "GitHub API token")
	fs.DurationVar(&githubCacheDuration, "github-cache-duration", githubCacheDuration, "GitHub API cache duration")
	if err := fs.Parse(os.Args); err != nil {
		panic(err)
	}

	f, err := os.Open(configFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var links []*Link
	if err := yaml.NewDecoder(f).Decode(&links); err != nil {
		panic(err)
	}

	_ = f.Close()

	var group errgroup.Group
	for _, link := range links {
		if link.Description == "" {
			group.Go(func() error {
				return link.FetchGitHubDescription()
			})
		}
	}
	if err := group.Wait(); err != nil {
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
