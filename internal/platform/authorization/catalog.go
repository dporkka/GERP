package authorization

import (
	"fmt"
	"sort"

	"gerp/internal/platform/resource"
)

type Permission struct {
	Code        string
	Description string
}

// FromResources derives the stable RBAC vocabulary used by HTTP, CLI, admin,
// MCP and automation surfaces. Explicit domain actions remain the source of
// truth for command permissions; generated CRUD permissions are emitted only
// for resources that opt into generic CRUD.
func FromResources(resources []resource.Resource) ([]Permission, error) {
	catalog := make(map[string]Permission)

	add := func(permission Permission) error {
		if permission.Code == "" {
			return fmt.Errorf("permission code is required")
		}
		if existing, ok := catalog[permission.Code]; ok && existing.Description != permission.Description {
			return fmt.Errorf("permission %s has conflicting descriptions", permission.Code)
		}
		catalog[permission.Code] = permission
		return nil
	}

	for _, definition := range resources {
		if err := definition.Validate(); err != nil {
			return nil, err
		}
		if definition.PermissionNS == "" {
			return nil, fmt.Errorf("resource %s: permission namespace is required", definition.Name)
		}

		if err := add(Permission{Code: definition.PermissionNS + ".read", Description: "Read " + definition.PluralLabel}); err != nil {
			return nil, err
		}

		if definition.MutationMode == resource.MutationModeCRUD {
			for _, operation := range []string{"create", "update", "archive", "restore"} {
				if err := add(Permission{
					Code:        definition.PermissionNS + "." + operation,
					Description: operation + " " + definition.PluralLabel,
				}); err != nil {
					return nil, err
				}
			}
		}

		for _, action := range definition.Actions {
			if action.Permission == "" {
				return nil, fmt.Errorf("resource %s action %s: permission is required", definition.Name, action.Name)
			}
			if err := add(Permission{Code: action.Permission, Description: action.Label}); err != nil {
				return nil, err
			}
		}
	}

	codes := make([]string, 0, len(catalog))
	for code := range catalog {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	permissions := make([]Permission, 0, len(codes))
	for _, code := range codes {
		permissions = append(permissions, catalog[code])
	}
	return permissions, nil
}
