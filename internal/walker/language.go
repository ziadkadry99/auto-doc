package walker

import (
	"path/filepath"
	"strings"
)

// extensionToLanguage maps file extensions to language names.
var extensionToLanguage = map[string]string{
	// Go
	".go": "Go",
	// Python
	".py":  "Python",
	".pyi": "Python",
	// TypeScript
	".ts":  "TypeScript",
	".tsx": "TypeScript",
	".mts": "TypeScript",
	// JavaScript
	".js":  "JavaScript",
	".jsx": "JavaScript",
	".mjs": "JavaScript",
	".cjs": "JavaScript",
	// Java
	".java": "Java",
	// Rust
	".rs": "Rust",
	// C
	".c": "C",
	".h": "C",
	// C++
	".cpp": "C++",
	".cc":  "C++",
	".cxx": "C++",
	".hpp": "C++",
	".hxx": "C++",
	// C#
	".cs": "C#",
	// Ruby
	".rb": "Ruby",
	// PHP
	".php": "PHP",
	// Swift
	".swift": "Swift",
	// Kotlin
	".kt":  "Kotlin",
	".kts": "Kotlin",
	// Scala
	".scala": "Scala",
	".sc":    "Scala",
	// Shell/Bash
	".sh":   "Shell",
	".bash": "Shell",
	".zsh":  "Shell",
	// SQL
	".sql": "SQL",
	// HTML
	".html": "HTML",
	".htm":  "HTML",
	// CSS
	".css":  "CSS",
	".scss": "CSS",
	".sass": "CSS",
	".less": "CSS",
	// YAML
	".yaml": "YAML",
	".yml":  "YAML",
	// JSON
	".json": "JSON",
	// TOML
	".toml": "TOML",
	// Terraform
	".tf":     "Terraform",
	".tfvars": "Terraform",
	// Markdown
	".md":       "Markdown",
	".markdown": "Markdown",
	// Protobuf
	".proto": "Protobuf",
	// Lua
	".lua": "Lua",
	// R
	".r": "R",
	".R": "R",
	// Dart
	".dart": "Dart",
	// Elixir
	".ex":  "Elixir",
	".exs": "Elixir",
	// Haskell
	".hs": "Haskell",
	// Perl
	".pl": "Perl",
	".pm": "Perl",
	// Vue
	".vue": "Vue",
	// Svelte
	".svelte": "Svelte",
}

// filenameToLanguage maps specific filenames to language names.
var filenameToLanguage = map[string]string{
	"Dockerfile":     "Dockerfile",
	"Makefile":       "Makefile",
	"Jenkinsfile":    "Groovy",
	"Vagrantfile":    "Ruby",
	"Gemfile":        "Ruby",
	"Rakefile":       "Ruby",
	".gitignore":     "Git",
	".dockerignore":  "Docker",
	"docker-compose.yml":  "YAML",
	"docker-compose.yaml": "YAML",
}

// DetectLanguage returns the programming language for a given filename
// based on its extension or exact filename. Returns "unknown" for
// unrecognized files.
func DetectLanguage(filename string) string {
	base := filepath.Base(filename)

	// Check exact filename matches first.
	if lang, ok := filenameToLanguage[base]; ok {
		return lang
	}

	ext := strings.ToLower(filepath.Ext(base))
	if ext == "" {
		return "unknown"
	}

	if lang, ok := extensionToLanguage[ext]; ok {
		return lang
	}

	return "unknown"
}
