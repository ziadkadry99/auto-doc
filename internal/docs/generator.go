package docs

import (
	"os"
	"path/filepath"
	"strings"
	"text/template"

	bizctx "github.com/ziadkadry99/auto-doc/internal/context"
	"github.com/ziadkadry99/auto-doc/internal/indexer"
)

// DocGenerator renders analysis results into markdown documentation files.
type DocGenerator struct {
	OutputDir       string
	BusinessContext *bizctx.BusinessContext
}

// NewDocGenerator creates a DocGenerator that writes to the given output directory.
func NewDocGenerator(outputDir string) *DocGenerator {
	return &DocGenerator{OutputDir: outputDir}
}

// GenerateFileDocs renders a markdown doc for each file analysis and writes it
// to {OutputDir}/docs/{relative_path}.md, mirroring the source tree structure.
func (g *DocGenerator) GenerateFileDocs(analyses []indexer.FileAnalysis) error {
	tmpl, err := template.New("filedoc").Funcs(templateFuncs).Parse(fileDocTemplate)
	if err != nil {
		return err
	}

	docsDir := filepath.Join(g.OutputDir, "docs")
	for _, a := range analyses {
		outPath := filepath.Join(docsDir, a.FilePath+".md")
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}

		f, err := os.Create(outPath)
		if err != nil {
			return err
		}

		err = tmpl.Execute(f, a)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// GenerateIndex renders an index.md listing all documented files.
func (g *DocGenerator) GenerateIndex(analyses []indexer.FileAnalysis) error {
	tmpl, err := template.New("index").Funcs(templateFuncs).Parse(indexTemplate)
	if err != nil {
		return err
	}

	docsDir := filepath.Join(g.OutputDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return err
	}

	outPath := filepath.Join(docsDir, "index.md")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	type quickLink struct {
		Label string
		Href  string
	}

	data := struct {
		ProjectName string
		Summary     string
		Files       []indexer.FileAnalysis
		QuickLinks  []quickLink
	}{
		ProjectName: projectNameFromWd(g.OutputDir),
		Files:       analyses,
		QuickLinks: []quickLink{
			{Label: "Architecture", Href: "architecture.md"},
		},
	}

	return tmpl.Execute(f, data)
}

// templateFuncs provides helper functions for the markdown templates.
var templateFuncs = template.FuncMap{
	"anchorize": anchorize,
	"code": func(s string) string {
		if s == "" {
			return ""
		}
		return "`" + s + "`"
	},
	"mdlink": func(filePath string) string {
		return filePath + ".md"
	},
	"oneline": func(s string) string {
		s = strings.ReplaceAll(s, "\n", " ")
		s = strings.ReplaceAll(s, "\r", "")
		return strings.TrimSpace(s)
	},
}

// projectNameFromWd returns the current working directory's base name as the
// project name. Falls back to filepath.Base(fallback) if Getwd fails.
func projectNameFromWd(fallback string) string {
	if wd, err := os.Getwd(); err == nil {
		return filepath.Base(wd)
	}
	return filepath.Base(fallback)
}

// anchorize converts a heading into a GitHub-style markdown anchor.
func anchorize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	var out strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			out.WriteRune(c)
		}
	}
	return out.String()
}
