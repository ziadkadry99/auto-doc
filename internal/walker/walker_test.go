package walker

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// testdataDir returns the absolute path to the testdata/sample_project directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	// Navigate from internal/walker to project root.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file location")
	}
	walkerDir := filepath.Dir(filename)
	root := filepath.Join(walkerDir, "..", "..", "testdata", "sample_project")
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("resolve testdata path: %v", err)
	}
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		t.Fatalf("testdata dir does not exist: %s", abs)
	}
	return abs
}

func TestWalk_BasicTraversal(t *testing.T) {
	dir := testdataDir(t)

	files, err := Walk(WalkerConfig{RootDir: dir})
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("Walk() returned no files")
	}

	// We expect at least: main.go, config.yaml, Dockerfile, utils.py, auth/middleware.go
	expectedFiles := map[string]bool{
		"main.go":            false,
		"config.yaml":       false,
		"Dockerfile":        false,
		"utils.py":          false,
		"auth/middleware.go": false,
	}

	for _, f := range files {
		if _, ok := expectedFiles[f.RelPath]; ok {
			expectedFiles[f.RelPath] = true
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("expected file %q not found in walk results", name)
		}
	}
}

func TestWalk_FileInfoFields(t *testing.T) {
	dir := testdataDir(t)

	files, err := Walk(WalkerConfig{RootDir: dir})
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	for _, f := range files {
		if f.Path == "" {
			t.Error("FileInfo.Path is empty")
		}
		if f.RelPath == "" {
			t.Error("FileInfo.RelPath is empty")
		}
		if f.Size <= 0 {
			t.Errorf("FileInfo.Size for %s is %d, expected > 0", f.RelPath, f.Size)
		}
		if f.Language == "" {
			t.Errorf("FileInfo.Language for %s is empty", f.RelPath)
		}
		if f.ContentHash == "" {
			t.Errorf("FileInfo.ContentHash for %s is empty", f.RelPath)
		}
		if len(f.ContentHash) != 64 {
			t.Errorf("FileInfo.ContentHash for %s has length %d, expected 64", f.RelPath, len(f.ContentHash))
		}
	}
}

func TestWalk_IncludeFilter(t *testing.T) {
	dir := testdataDir(t)

	files, err := Walk(WalkerConfig{
		RootDir: dir,
		Include: []string{"*.go"},
	})
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	for _, f := range files {
		if !strings.HasSuffix(f.RelPath, ".go") {
			t.Errorf("include filter *.go let through: %s", f.RelPath)
		}
	}

	if len(files) < 2 {
		t.Errorf("expected at least 2 .go files, got %d", len(files))
	}
}

func TestWalk_ExcludeFilter(t *testing.T) {
	dir := testdataDir(t)

	files, err := Walk(WalkerConfig{
		RootDir: dir,
		Exclude: []string{"*.py"},
	})
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	for _, f := range files {
		if strings.HasSuffix(f.RelPath, ".py") {
			t.Errorf("exclude filter *.py did not exclude: %s", f.RelPath)
		}
	}
}

func TestWalk_DoubleStarInclude(t *testing.T) {
	dir := testdataDir(t)

	files, err := Walk(WalkerConfig{
		RootDir: dir,
		Include: []string{"**/*.go"},
	})
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	foundNested := false
	for _, f := range files {
		if strings.Contains(f.RelPath, "/") {
			foundNested = true
		}
		if !strings.HasSuffix(f.RelPath, ".go") {
			t.Errorf("include filter **/*.go let through: %s", f.RelPath)
		}
	}

	if !foundNested {
		t.Error("expected **/*.go to match nested Go files")
	}
}

func TestWalk_SkipsBinaryFiles(t *testing.T) {
	// Create a temp dir with a binary file.
	tmpDir := t.TempDir()

	// Write a text file.
	os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# Hello"), 0644)

	// Write a binary file (contains NUL bytes).
	binary := make([]byte, 100)
	binary[50] = 0x00
	os.WriteFile(filepath.Join(tmpDir, "image.bin"), binary, 0644)

	files, err := Walk(WalkerConfig{RootDir: tmpDir})
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	for _, f := range files {
		if f.RelPath == "image.bin" {
			t.Error("binary file image.bin should have been skipped")
		}
	}

	if len(files) != 1 {
		t.Errorf("expected 1 file (readme.md), got %d", len(files))
	}
}

func TestWalk_SkipsLargeFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a small file.
	os.WriteFile(filepath.Join(tmpDir, "small.txt"), []byte("small"), 0644)

	// Write a file that exceeds our small limit.
	big := make([]byte, 200)
	for i := range big {
		big[i] = 'A'
	}
	os.WriteFile(filepath.Join(tmpDir, "big.txt"), big, 0644)

	files, err := Walk(WalkerConfig{
		RootDir:     tmpDir,
		MaxFileSize: 100, // 100 bytes
	})
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	for _, f := range files {
		if f.RelPath == "big.txt" {
			t.Error("big.txt should have been skipped (exceeds MaxFileSize)")
		}
	}
}

func TestWalk_DefaultExcludeDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directories that should be excluded.
	for _, dir := range []string{"node_modules", ".git", "vendor", "__pycache__"} {
		dirPath := filepath.Join(tmpDir, dir)
		os.MkdirAll(dirPath, 0755)
		os.WriteFile(filepath.Join(dirPath, "file.js"), []byte("content"), 0644)
	}

	// Create a normal file.
	os.WriteFile(filepath.Join(tmpDir, "app.js"), []byte("const x = 1;"), 0644)

	files, err := Walk(WalkerConfig{RootDir: tmpDir})
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	if len(files) != 1 {
		names := make([]string, len(files))
		for i, f := range files {
			names[i] = f.RelPath
		}
		t.Errorf("expected 1 file, got %d: %v", len(files), names)
	}
}

func TestWalk_Gitignore(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gitignore.
	os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("*.log\nsecret.txt\n"), 0644)

	// Create files.
	os.WriteFile(filepath.Join(tmpDir, "app.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "debug.log"), []byte("log data"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "secret.txt"), []byte("password"), 0644)

	files, err := Walk(WalkerConfig{RootDir: tmpDir})
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	relPaths := make([]string, len(files))
	for i, f := range files {
		relPaths[i] = f.RelPath
	}
	sort.Strings(relPaths)

	// .gitignore itself and app.go should remain; debug.log and secret.txt should be ignored.
	for _, excluded := range []string{"debug.log", "secret.txt"} {
		for _, rp := range relPaths {
			if rp == excluded {
				t.Errorf("file %q should be excluded by .gitignore", excluded)
			}
		}
	}

	foundApp := false
	for _, rp := range relPaths {
		if rp == "app.go" {
			foundApp = true
		}
	}
	if !foundApp {
		t.Error("app.go should not be excluded")
	}
}

func TestWalk_ContentHashConsistency(t *testing.T) {
	dir := testdataDir(t)

	files1, err := Walk(WalkerConfig{RootDir: dir})
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	files2, err := Walk(WalkerConfig{RootDir: dir})
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	hash1 := make(map[string]string)
	for _, f := range files1 {
		hash1[f.RelPath] = f.ContentHash
	}

	for _, f := range files2 {
		if h, ok := hash1[f.RelPath]; ok {
			if h != f.ContentHash {
				t.Errorf("content hash mismatch for %s: %s vs %s", f.RelPath, h, f.ContentHash)
			}
		}
	}
}

// --- Language detection tests ---

func TestDetectLanguage_Extensions(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"main.go", "Go"},
		{"app.py", "Python"},
		{"index.ts", "TypeScript"},
		{"index.tsx", "TypeScript"},
		{"app.js", "JavaScript"},
		{"Main.java", "Java"},
		{"lib.rs", "Rust"},
		{"main.c", "C"},
		{"util.h", "C"},
		{"main.cpp", "C++"},
		{"main.cc", "C++"},
		{"Program.cs", "C#"},
		{"app.rb", "Ruby"},
		{"index.php", "PHP"},
		{"App.swift", "Swift"},
		{"Main.kt", "Kotlin"},
		{"App.scala", "Scala"},
		{"script.sh", "Shell"},
		{"query.sql", "SQL"},
		{"page.html", "HTML"},
		{"style.css", "CSS"},
		{"style.scss", "CSS"},
		{"config.yaml", "YAML"},
		{"config.yml", "YAML"},
		{"data.json", "JSON"},
		{"config.toml", "TOML"},
		{"main.tf", "Terraform"},
		{"README.md", "Markdown"},
		{"schema.proto", "Protobuf"},
		{"App.vue", "Vue"},
		{"Page.svelte", "Svelte"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			got := DetectLanguage(tc.filename)
			if got != tc.want {
				t.Errorf("DetectLanguage(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

func TestDetectLanguage_SpecialFilenames(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"Dockerfile", "Dockerfile"},
		{"Makefile", "Makefile"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			got := DetectLanguage(tc.filename)
			if got != tc.want {
				t.Errorf("DetectLanguage(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

func TestDetectLanguage_Unknown(t *testing.T) {
	got := DetectLanguage("noextension")
	if got != "unknown" {
		t.Errorf("DetectLanguage(noextension) = %q, want %q", got, "unknown")
	}

	got = DetectLanguage("file.xyz")
	if got != "unknown" {
		t.Errorf("DetectLanguage(file.xyz) = %q, want %q", got, "unknown")
	}
}

func TestDetectLanguage_WithPath(t *testing.T) {
	got := DetectLanguage("src/components/App.tsx")
	if got != "TypeScript" {
		t.Errorf("DetectLanguage with path = %q, want TypeScript", got)
	}
}

// --- Filter tests ---

func TestMatchesInclude_Empty(t *testing.T) {
	if !MatchesInclude("anything.go", nil) {
		t.Error("empty include patterns should include everything")
	}
}

func TestMatchesInclude_Pattern(t *testing.T) {
	if !MatchesInclude("main.go", []string{"*.go"}) {
		t.Error("*.go should match main.go")
	}
	if MatchesInclude("main.py", []string{"*.go"}) {
		t.Error("*.go should not match main.py")
	}
}

func TestMatchesExclude_Empty(t *testing.T) {
	if MatchesExclude("anything.go", nil) {
		t.Error("empty exclude patterns should exclude nothing")
	}
}

func TestMatchesExclude_Pattern(t *testing.T) {
	if !MatchesExclude("debug.log", []string{"*.log"}) {
		t.Error("*.log should match debug.log")
	}
	if MatchesExclude("main.go", []string{"*.log"}) {
		t.Error("*.log should not match main.go")
	}
}

func TestMatchesInclude_DoubleStarPattern(t *testing.T) {
	if !MatchesInclude("src/auth/middleware.go", []string{"**/*.go"}) {
		t.Error("**/*.go should match src/auth/middleware.go")
	}
}

// --- Test file detection ---

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		name    string
		relPath string
		want    bool
	}{
		{"walker_test.go", "internal/walker/walker_test.go", true},
		{"test_utils.py", "test_utils.py", true},
		{"app.test.js", "src/app.test.js", true},
		{"app.spec.ts", "src/app.spec.ts", true},
		{"main.go", "main.go", false},
		{"utils.py", "utils.py", false},
		{"test/helper.go", "test/helper.go", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isTestFile(tc.name, tc.relPath)
			if got != tc.want {
				t.Errorf("isTestFile(%q, %q) = %v, want %v", tc.name, tc.relPath, got, tc.want)
			}
		})
	}
}

func TestWalk_LanguageDetectionInResults(t *testing.T) {
	dir := testdataDir(t)

	files, err := Walk(WalkerConfig{RootDir: dir})
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	expected := map[string]string{
		"main.go":            "Go",
		"config.yaml":        "YAML",
		"Dockerfile":         "Dockerfile",
		"utils.py":           "Python",
		"auth/middleware.go":  "Go",
	}

	found := make(map[string]string)
	for _, f := range files {
		found[f.RelPath] = f.Language
	}

	for path, wantLang := range expected {
		gotLang, ok := found[path]
		if !ok {
			t.Errorf("file %q not found in results", path)
			continue
		}
		if gotLang != wantLang {
			t.Errorf("language for %q: got %q, want %q", path, gotLang, wantLang)
		}
	}
}
