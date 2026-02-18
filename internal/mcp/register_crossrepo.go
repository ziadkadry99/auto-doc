package mcp

// registerCrossRepoTools adds the cross-repo tool definitions and handlers to the MCP server.
func (s *Server) registerCrossRepoTools() {
	s.mcp.AddTool(searchAcrossReposTool, s.handleSearchAcrossRepos)
	s.mcp.AddTool(getServiceContextTool, s.handleGetServiceContext)
	s.mcp.AddTool(getBlastRadiusTool, s.handleGetBlastRadius)
	s.mcp.AddTool(getFlowTool, s.handleGetFlow)
	s.mcp.AddTool(askArchitectureTool, s.handleAskArchitecture)
	s.mcp.AddTool(getTeamServicesTool, s.handleGetTeamServices)
	s.mcp.AddTool(provideContextTool, s.handleProvideContext)
	s.mcp.AddTool(listReposTool, s.handleListRepos)
	s.mcp.AddTool(getRepoDetailsTool, s.handleGetRepoDetails)
	s.mcp.AddTool(getSystemDiagramTool, s.handleGetSystemDiagram)
	s.mcp.AddTool(getServiceMapDataTool, s.handleGetServiceMapData)
}
