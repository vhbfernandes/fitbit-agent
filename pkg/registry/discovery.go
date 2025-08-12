package registry

import (
	"fmt"
	"reflect"

	"github.com/vhbfernandes/fitbit-agent/pkg/agent"
)

// ToolDiscovery handles automatic tool registration
type ToolDiscovery struct {
	registry agent.ToolRegistry
}

// NewToolDiscovery creates a new tool discovery instance
func NewToolDiscovery(registry agent.ToolRegistry) *ToolDiscovery {
	return &ToolDiscovery{
		registry: registry,
	}
}

// AutoRegisterTools automatically registers tools that implement the Tool interface
func (td *ToolDiscovery) AutoRegisterTools(tools ...interface{}) error {
	for _, tool := range tools {
		if t, ok := tool.(agent.Tool); ok {
			td.registry.RegisterTool(t)
			fmt.Printf("Registered tool: %s\n", t.Name())
		} else {
			return fmt.Errorf("object %v does not implement agent.Tool interface", reflect.TypeOf(tool))
		}
	}
	return nil
}

// RegisterToolFactories registers tool factories for lazy initialization
func (td *ToolDiscovery) RegisterToolFactories(factories ...func() agent.Tool) {
	for _, factory := range factories {
		tool := factory()
		td.registry.RegisterTool(tool)
		fmt.Printf("Registered tool: %s\n", tool.Name())
	}
}
