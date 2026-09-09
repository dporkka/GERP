package resource

import (
	"errors"
	"fmt"
	"sort"
)

var ErrDuplicateResource = errors.New("duplicate resource")

type MutationMode string

const (
	MutationModeCRUD         MutationMode = "crud"
	MutationModeCommandsOnly MutationMode = "commands_only"
	MutationModeReadOnly     MutationMode = "read_only"
)

type FieldKind string

const (
	FieldString   FieldKind = "string"
	FieldText     FieldKind = "text"
	FieldInteger  FieldKind = "integer"
	FieldMoney    FieldKind = "money"
	FieldDecimal  FieldKind = "decimal"
	FieldBoolean  FieldKind = "boolean"
	FieldDate     FieldKind = "date"
	FieldDateTime FieldKind = "datetime"
	FieldUUID     FieldKind = "uuid"
	FieldEnum     FieldKind = "enum"
	FieldJSON     FieldKind = "json"
)

type Field struct {
	Name       string
	Label      string
	Kind       FieldKind
	Required   bool
	Searchable bool
	Sortable   bool
	Filterable bool
	Sensitive  bool
	ReadOnly   bool
	Options    []string
}

type Action struct {
	Name       string
	Label      string
	Permission string
	Command    string
	Dangerous  bool
}

type Resource struct {
	Name          string
	Label         string
	PluralLabel   string
	Module        string
	PermissionNS  string
	MutationMode  MutationMode
	TenantScoped  bool
	Audited       bool
	Fields        []Field
	Actions       []Action
	DefaultSort   string
	DefaultSearch []string
}

func (r Resource) Validate() error {
	if r.Name == "" {
		return errors.New("resource name is required")
	}
	if r.Module == "" {
		return fmt.Errorf("resource %s: module is required", r.Name)
	}
	if r.MutationMode == "" {
		return fmt.Errorf("resource %s: mutation mode is required", r.Name)
	}

	seen := make(map[string]struct{}, len(r.Fields))
	for _, field := range r.Fields {
		if field.Name == "" {
			return fmt.Errorf("resource %s: field name is required", r.Name)
		}
		if _, ok := seen[field.Name]; ok {
			return fmt.Errorf("resource %s: duplicate field %s", r.Name, field.Name)
		}
		seen[field.Name] = struct{}{}
	}

	if r.MutationMode == MutationModeCommandsOnly && len(r.Actions) == 0 {
		return fmt.Errorf("resource %s: commands-only resource requires explicit actions", r.Name)
	}
	return nil
}

type Registry struct {
	resources map[string]Resource
}

func NewRegistry() *Registry {
	return &Registry{resources: make(map[string]Resource)}
}

func (r *Registry) Register(resource Resource) error {
	if err := resource.Validate(); err != nil {
		return err
	}
	if _, ok := r.resources[resource.Name]; ok {
		return fmt.Errorf("%w: %s", ErrDuplicateResource, resource.Name)
	}
	r.resources[resource.Name] = resource
	return nil
}

func (r *Registry) Get(name string) (Resource, bool) {
	resource, ok := r.resources[name]
	return resource, ok
}

func (r *Registry) List() []Resource {
	names := make([]string, 0, len(r.resources))
	for name := range r.resources {
		names = append(names, name)
	}
	sort.Strings(names)

	resources := make([]Resource, 0, len(names))
	for _, name := range names {
		resources = append(resources, r.resources[name])
	}
	return resources
}
