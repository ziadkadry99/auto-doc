package progress

import (
	"fmt"
	"os"

	"github.com/schollz/progressbar/v3"
)

// Reporter provides progress feedback during documentation generation.
type Reporter interface {
	Start(total int)
	Update(current int, message string)
	Finish()
}

// NewReporter returns a TerminalReporter if running in an interactive terminal,
// or a CIReporter if the CI environment variable is set.
func NewReporter() Reporter {
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		return &CIReporter{}
	}
	return &TerminalReporter{}
}

// TerminalReporter displays a progress bar in the terminal.
type TerminalReporter struct {
	bar *progressbar.ProgressBar
}

func (r *TerminalReporter) Start(total int) {
	r.bar = progressbar.NewOptions(total,
		progressbar.OptionSetDescription("Generating docs"),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
		progressbar.OptionClearOnFinish(),
	)
}

func (r *TerminalReporter) Update(current int, message string) {
	if r.bar != nil {
		r.bar.Describe(message)
		_ = r.bar.Set(current)
	}
}

func (r *TerminalReporter) Finish() {
	if r.bar != nil {
		_ = r.bar.Finish()
	}
}

// CIReporter prints line-by-line progress suitable for CI logs.
type CIReporter struct {
	total int
}

func (r *CIReporter) Start(total int) {
	r.total = total
	fmt.Fprintf(os.Stderr, "Starting documentation generation for %d files\n", total)
}

func (r *CIReporter) Update(current int, message string) {
	fmt.Fprintf(os.Stderr, "[%d/%d] %s\n", current, r.total, message)
}

func (r *CIReporter) Finish() {
	fmt.Fprintln(os.Stderr, "Documentation generation complete")
}
