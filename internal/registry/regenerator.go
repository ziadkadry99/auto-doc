package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/ziadkadry99/auto-doc/internal/docs"
	"github.com/ziadkadry99/auto-doc/internal/flows"
	"github.com/ziadkadry99/auto-doc/internal/llm"
)

// Regenerator orchestrates documentation regeneration after repo changes.
type Regenerator struct {
	store     *Store
	linker    *Linker
	flowStore *flows.Store
	outputDir string
}

// NewRegenerator creates a new regeneration orchestrator.
func NewRegenerator(store *Store, linker *Linker, flowStore *flows.Store, outputDir string) *Regenerator {
	return &Regenerator{
		store:     store,
		linker:    linker,
		flowStore: flowStore,
		outputDir: outputDir,
	}
}

// Regenerate runs after a repo is imported/synced to update cross-service documentation.
func (r *Regenerator) Regenerate(ctx context.Context, changedRepo string, provider llm.Provider, model string) error {
	var actions []string

	// 1. Get blast radius.
	affected, err := r.linker.GetAffectedRepos(ctx, changedRepo)
	if err != nil {
		return fmt.Errorf("getting affected repos: %w", err)
	}
	actions = append(actions, fmt.Sprintf("blast radius: %s affects [%s]", changedRepo, strings.Join(affected, ", ")))

	// 2. Re-discover links for the changed repo.
	repo, err := r.store.Get(ctx, changedRepo)
	if err != nil || repo == nil {
		return fmt.Errorf("getting repo %s: %w", changedRepo, err)
	}

	if provider != nil {
		if err := r.linker.DiscoverLinks(ctx, repo, provider, model); err != nil {
			actions = append(actions, fmt.Sprintf("link discovery failed: %v", err))
		} else {
			actions = append(actions, "link discovery: updated")
		}
	}

	// 3. Load all repos and links.
	allRepos, err := r.store.List(ctx)
	if err != nil {
		return fmt.Errorf("listing repos: %w", err)
	}

	allLinks, err := r.store.GetLinks(ctx, "")
	if err != nil {
		return fmt.Errorf("getting links: %w", err)
	}

	// Convert to docs types.
	docRepos := reposToServiceInfo(allRepos)
	docLinks := linksToServiceLinkInfo(allLinks)

	// 4. Regenerate system architecture diagram.
	_, err = docs.GenerateSystemDiagram(ctx, docRepos, docLinks, provider, model)
	if err != nil {
		actions = append(actions, fmt.Sprintf("system diagram failed: %v", err))
	} else {
		actions = append(actions, "system diagram: regenerated")
	}

	// 5. Regenerate service-level interactive map.
	projectName := "System"
	if len(allRepos) > 0 {
		projectName = allRepos[0].DisplayName + " System"
	}
	if err := docs.GenerateServiceMap(r.outputDir, docRepos, docLinks, projectName); err != nil {
		actions = append(actions, fmt.Sprintf("service map failed: %v", err))
	} else {
		actions = append(actions, "service map: regenerated")
	}

	// 6. Regenerate system overview document.
	var allFlows []flows.Flow
	if r.flowStore != nil {
		allFlows, _ = r.flowStore.ListFlows(ctx)
	}

	if err := docs.GenerateSystemOverview(ctx, r.outputDir, docRepos, docLinks, allFlows, provider, model); err != nil {
		actions = append(actions, fmt.Sprintf("system overview failed: %v", err))
	} else {
		actions = append(actions, "system overview: regenerated")
	}

	// Log what was regenerated.
	fmt.Printf("Regeneration complete for %s:\n", changedRepo)
	for _, action := range actions {
		fmt.Printf("  - %s\n", action)
	}

	return nil
}

// reposToServiceInfo converts registry repos to docs ServiceInfo type.
func reposToServiceInfo(repos []Repository) []docs.ServiceInfo {
	result := make([]docs.ServiceInfo, len(repos))
	for i, r := range repos {
		result[i] = docs.ServiceInfo{
			Name:        r.Name,
			DisplayName: r.DisplayName,
			Summary:     r.Summary,
			FileCount:   r.FileCount,
			SourceType:  r.SourceType,
			Status:      r.Status,
		}
	}
	return result
}

// linksToServiceLinkInfo converts registry links to docs ServiceLinkInfo type.
func linksToServiceLinkInfo(links []ServiceLink) []docs.ServiceLinkInfo {
	result := make([]docs.ServiceLinkInfo, len(links))
	for i, l := range links {
		result[i] = docs.ServiceLinkInfo{
			FromRepo:  l.FromRepo,
			ToRepo:    l.ToRepo,
			LinkType:  l.LinkType,
			Reason:    l.Reason,
			Endpoints: l.Endpoints,
		}
	}
	return result
}
