package knowledgegraph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RunMCPServer(ctx context.Context, svc *Service) error {
	if svc == nil {
		return fmt.Errorf("knowledgegraph: service is nil")
	}
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "KGgraph",
		Version: "0.1.2",
	}, nil)
	for _, toolName := range svc.ToolNames() {
		t := &mcp.Tool{
			Name:        toolName,
			Description: ToolDescription(toolName),
			InputSchema: ToolSchema(toolName),
		}
		name := toolName
		server.AddTool(t, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args map[string]any
			if len(req.Params.Arguments) > 0 {
				if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
					return &mcp.CallToolResult{
						Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(`{"error":%q}`, err.Error())}},
						IsError: true,
					}, nil
				}
			}
			out, err := svc.Call(ctx, name, args)
			if err != nil {
				payload := map[string]any{"error": err.Error()}
				raw, _ := json.Marshal(payload)
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}},
					IsError: true,
				}, nil
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return nil, err
			}
			return &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: string(raw)}},
				StructuredContent: out,
			}, nil
		})
	}
	return server.Run(ctx, &mcp.StdioTransport{})
}
