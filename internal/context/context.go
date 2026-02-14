package context

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BusinessContext holds optional project-level context provided by the
// maintainer. It is used to enrich LLM prompts during documentation
// generation.
type BusinessContext struct {
	Description    string `json:"description,omitempty"`
	TargetUsers    string `json:"target_users,omitempty"`
	KeyConcepts    string `json:"key_concepts,omitempty"`
	ArchDecisions  string `json:"arch_decisions,omitempty"`
	AdditionalInfo string `json:"additional_info,omitempty"`
}

// Load reads a BusinessContext from a JSON file. Returns nil and no error if
// the file does not exist.
func Load(path string) (*BusinessContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading context file: %w", err)
	}

	var ctx BusinessContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("parsing context file: %w", err)
	}

	if ctx.IsEmpty() {
		return nil, nil
	}
	return &ctx, nil
}

// Save writes the BusinessContext to a JSON file, creating parent directories
// as needed.
func (c *BusinessContext) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating context directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling context: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing context file: %w", err)
	}
	return nil
}

// IsEmpty returns true if no fields are populated.
func (c *BusinessContext) IsEmpty() bool {
	return c.Description == "" &&
		c.TargetUsers == "" &&
		c.KeyConcepts == "" &&
		c.ArchDecisions == "" &&
		c.AdditionalInfo == ""
}

// ToPromptSection formats the context as a text block suitable for injection
// into an LLM prompt.
func (c *BusinessContext) ToPromptSection() string {
	if c.IsEmpty() {
		return ""
	}

	var b strings.Builder
	if c.Description != "" {
		fmt.Fprintf(&b, "Project description: %s\n", c.Description)
	}
	if c.TargetUsers != "" {
		fmt.Fprintf(&b, "Target users: %s\n", c.TargetUsers)
	}
	if c.KeyConcepts != "" {
		fmt.Fprintf(&b, "Key business domains/concepts: %s\n", c.KeyConcepts)
	}
	if c.ArchDecisions != "" {
		fmt.Fprintf(&b, "Architectural decisions: %s\n", c.ArchDecisions)
	}
	if c.AdditionalInfo != "" {
		fmt.Fprintf(&b, "Additional context: %s\n", c.AdditionalInfo)
	}
	return b.String()
}
