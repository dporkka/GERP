package module

import (
	"errors"
	"reflect"
	"testing"
)

type testComponent struct {
	desc Descriptor
}

func (c testComponent) Descriptor() Descriptor { return c.desc }

func TestRegistryResolveOrdersDependencies(t *testing.T) {
	registry := NewRegistry()
	components := []Component{
		testComponent{desc: Descriptor{Name: "sales", DependsOn: []string{"crm", "finance"}}},
		testComponent{desc: Descriptor{Name: "core"}},
		testComponent{desc: Descriptor{Name: "finance", DependsOn: []string{"core"}}},
		testComponent{desc: Descriptor{Name: "crm", DependsOn: []string{"core"}}},
	}

	for _, component := range components {
		if err := registry.Register(component); err != nil {
			t.Fatalf("register module: %v", err)
		}
	}

	resolved, err := registry.Resolve()
	if err != nil {
		t.Fatalf("resolve modules: %v", err)
	}

	got := make([]string, 0, len(resolved))
	for _, component := range resolved {
		got = append(got, component.Descriptor().Name)
	}
	want := []string{"core", "crm", "finance", "sales"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected order: got %v want %v", got, want)
	}
}

func TestRegistryRejectsDuplicateModule(t *testing.T) {
	registry := NewRegistry()
	component := testComponent{desc: Descriptor{Name: "finance"}}
	if err := registry.Register(component); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := registry.Register(component); !errors.Is(err, ErrDuplicateModule) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestRegistryRejectsMissingDependency(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(testComponent{desc: Descriptor{Name: "sales", DependsOn: []string{"crm"}}}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := registry.Resolve(); !errors.Is(err, ErrMissingDependency) {
		t.Fatalf("expected missing dependency error, got %v", err)
	}
}

func TestRegistryRejectsDependencyCycle(t *testing.T) {
	registry := NewRegistry()
	for _, component := range []Component{
		testComponent{desc: Descriptor{Name: "a", DependsOn: []string{"b"}}},
		testComponent{desc: Descriptor{Name: "b", DependsOn: []string{"a"}}},
	} {
		if err := registry.Register(component); err != nil {
			t.Fatalf("register: %v", err)
		}
	}

	if _, err := registry.Resolve(); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("expected cycle error, got %v", err)
	}
}
