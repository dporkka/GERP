package module

import (
	"errors"
	"fmt"
	"sort"
)

var (
	ErrDuplicateModule   = errors.New("duplicate module")
	ErrMissingDependency = errors.New("missing module dependency")
	ErrDependencyCycle   = errors.New("module dependency cycle")
)

// Descriptor is the stable metadata contract for a Gerp module.
// Dependencies are module names, not package paths, so application composition
// remains independent from persistence and transport implementations.
type Descriptor struct {
	Name        string
	Version     string
	Description string
	DependsOn   []string
	Provides    []string
}

// Component is the smallest unit the Gerp composition root understands.
// Runtime-specific wiring belongs behind module-owned constructors rather than
// inside the registry.
type Component interface {
	Descriptor() Descriptor
}

// Registry collects modules and resolves a deterministic dependency order.
type Registry struct {
	components map[string]Component
}

func NewRegistry() *Registry {
	return &Registry{components: make(map[string]Component)}
}

func (r *Registry) Register(component Component) error {
	if component == nil {
		return errors.New("module component is nil")
	}

	desc := component.Descriptor()
	if desc.Name == "" {
		return errors.New("module name is required")
	}
	if _, exists := r.components[desc.Name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateModule, desc.Name)
	}

	r.components[desc.Name] = component
	return nil
}

func (r *Registry) Get(name string) (Component, bool) {
	component, ok := r.components[name]
	return component, ok
}

func (r *Registry) Resolve() ([]Component, error) {
	if len(r.components) == 0 {
		return nil, nil
	}

	for name, component := range r.components {
		for _, dependency := range component.Descriptor().DependsOn {
			if _, ok := r.components[dependency]; !ok {
				return nil, fmt.Errorf("%w: %s requires %s", ErrMissingDependency, name, dependency)
			}
		}
	}

	state := make(map[string]uint8, len(r.components))
	ordered := make([]Component, 0, len(r.components))

	names := make([]string, 0, len(r.components))
	for name := range r.components {
		names = append(names, name)
	}
	sort.Strings(names)

	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("%w: %s", ErrDependencyCycle, name)
		case 2:
			return nil
		}

		state[name] = 1
		component := r.components[name]
		dependencies := append([]string(nil), component.Descriptor().DependsOn...)
		sort.Strings(dependencies)
		for _, dependency := range dependencies {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[name] = 2
		ordered = append(ordered, component)
		return nil
	}

	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return ordered, nil
}
