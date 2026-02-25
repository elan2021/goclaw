package skill

import "fmt"

// Registry manages all available tools.
type Registry struct {
	tools map[string]Skill
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Skill)}
}

// Register adds a skill to the registry.
func (r *Registry) Register(s Skill) error {
	name := s.Definition().Name
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("skill %q already registered", name)
	}
	r.tools[name] = s
	return nil
}

// Get retrieves a skill by name.
func (r *Registry) Get(name string) (Skill, bool) {
	s, ok := r.tools[name]
	return s, ok
}

// Definitions returns all tool specs for the LLM.
func (r *Registry) Definitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, s := range r.tools {
		defs = append(defs, s.Definition())
	}
	return defs
}

// Count returns the number of registered skills.
func (r *Registry) Count() int {
	return len(r.tools)
}
