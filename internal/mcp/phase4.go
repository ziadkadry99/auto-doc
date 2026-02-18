package mcp

import (
	"github.com/ziadkadry99/auto-doc/internal/contextengine"
	"github.com/ziadkadry99/auto-doc/internal/flows"
	"github.com/ziadkadry99/auto-doc/internal/orgstructure"
	"github.com/ziadkadry99/auto-doc/internal/registry"
)

// Phase4Deps holds optional Phase 4 dependencies for cross-repo tools.
type Phase4Deps struct {
	CtxStore  *contextengine.Store
	CtxEngine *contextengine.Engine
	FlowStore *flows.Store
	OrgStore  *orgstructure.Store
	RepoStore *registry.Store
}

// SetPhase4Deps sets the optional Phase 4 dependencies and registers cross-repo tools.
func (s *Server) SetPhase4Deps(deps Phase4Deps) {
	s.phase4 = &deps
	s.registerCrossRepoTools()
}
