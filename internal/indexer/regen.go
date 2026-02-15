package indexer

import (
	"context"
	"fmt"
	"strings"

	"github.com/ziadkadry99/auto-doc/internal/llm"
)

// RegenerationAdvice holds the LLM's decision about which high-level docs to regenerate.
type RegenerationAdvice struct {
	ProjectOverview bool
	Architecture    bool
	FeaturePages    bool
	ComponentMap    bool
	Reasoning       string
}

// DecideRegeneration asks the LLM whether high-level documentation artifacts need
// regeneration based on the files that changed and their summaries.
// On any failure, it returns advice to regenerate everything (safe fallback).
func DecideRegeneration(
	ctx context.Context,
	provider llm.Provider,
	model string,
	directlyChanged []string,
	depAffected []string,
	allAnalyses map[string]FileAnalysis,
) (*RegenerationAdvice, error) {
	totalFiles := len(allAnalyses)
	changedCount := len(directlyChanged) + len(depAffected)

	// Build summaries of changed files for context.
	var summaryLines []string
	allChanged := append(directlyChanged, depAffected...)
	for _, f := range allChanged {
		if a, ok := allAnalyses[f]; ok && a.Summary != "" {
			summaryLines = append(summaryLines, fmt.Sprintf("- %s: %s", f, a.Summary))
		} else {
			summaryLines = append(summaryLines, fmt.Sprintf("- %s: (no summary available)", f))
		}
		if len(summaryLines) >= 30 {
			summaryLines = append(summaryLines, fmt.Sprintf("... and %d more files", len(allChanged)-30))
			break
		}
	}

	prompt := fmt.Sprintf(`You are a documentation maintenance assistant. Based on the changes described below, decide which high-level documentation artifacts need to be regenerated.

Total files in project: %d
Files changed in this update: %d (of which %d are directly changed, %d affected via dependencies)

Changed file summaries:
%s

For each artifact, answer YES or NO:
1. PROJECT_OVERVIEW - The main project summary/home page
2. ARCHITECTURE - The architecture overview document
3. FEATURE_PAGES - The feature grouping pages
4. COMPONENT_MAP - The interactive component map

Respond in exactly this format (one per line):
PROJECT_OVERVIEW: YES or NO
ARCHITECTURE: YES or NO
FEATURE_PAGES: YES or NO
COMPONENT_MAP: YES or NO
REASONING: <one sentence explaining your decision>`,
		totalFiles, changedCount, len(directlyChanged), len(depAffected),
		strings.Join(summaryLines, "\n"))

	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   256,
		Temperature: 0.1,
	})
	if err != nil {
		return regenEverything("LLM call failed: " + err.Error()), nil
	}

	advice := parseRegenerationAdvice(resp.Content)
	if advice == nil {
		return regenEverything("could not parse LLM response"), nil
	}
	return advice, nil
}

// parseRegenerationAdvice extracts structured advice from the LLM response text.
// Returns nil if the response cannot be parsed.
func parseRegenerationAdvice(response string) *RegenerationAdvice {
	advice := &RegenerationAdvice{}
	foundAny := false

	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		isYes := strings.HasPrefix(strings.ToUpper(val), "YES")

		switch strings.ToUpper(key) {
		case "PROJECT_OVERVIEW":
			advice.ProjectOverview = isYes
			foundAny = true
		case "ARCHITECTURE":
			advice.Architecture = isYes
			foundAny = true
		case "FEATURE_PAGES":
			advice.FeaturePages = isYes
			foundAny = true
		case "COMPONENT_MAP":
			advice.ComponentMap = isYes
			foundAny = true
		case "REASONING":
			advice.Reasoning = val
		}
	}

	if !foundAny {
		return nil
	}
	return advice
}

// regenEverything returns advice to regenerate all artifacts (safe fallback).
func regenEverything(reason string) *RegenerationAdvice {
	return &RegenerationAdvice{
		ProjectOverview: true,
		Architecture:    true,
		FeaturePages:    true,
		ComponentMap:    true,
		Reasoning:       fmt.Sprintf("Regenerating everything (fallback: %s)", reason),
	}
}
