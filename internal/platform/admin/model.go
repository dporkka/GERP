package admin

import (
	"fmt"
	"sort"

	"gerp/internal/platform/authorization"
	"gerp/internal/platform/resource"
)

type MenuItem struct {
	Module     string
	Resource   string
	Label      string
	Path       string
	Permission string
}

type Form struct {
	Resource   string
	Fields     []resource.Field
	Method     string
	Action     string
	Permission string
}

type Table struct {
	Resource          string
	Columns           []resource.Field
	RowActions        []resource.Action
	CollectionActions []resource.Action
}

type Model struct {
	Menu        []MenuItem
	Forms       map[string]Form
	Tables      map[string]Table
	Permissions []authorization.Permission
}

func Generate(resources []resource.Resource) (Model, error) {
	permissions, err := authorization.FromResources(resources)
	if err != nil {
		return Model{}, err
	}
	model := Model{
		Forms:       make(map[string]Form),
		Tables:      make(map[string]Table),
		Permissions: permissions,
	}
	for _, definition := range resources {
		if err := definition.Validate(); err != nil {
			return Model{}, err
		}
		model.Menu = append(model.Menu, MenuItem{
			Module:     definition.Module,
			Resource:   definition.Name,
			Label:      definition.PluralLabel,
			Path:       "/admin/" + definition.Name,
			Permission: definition.PermissionNS + ".read",
		})

		table := Table{Resource: definition.Name}
		for _, field := range definition.Fields {
			if !field.Sensitive {
				table.Columns = append(table.Columns, field)
			}
		}
		for _, action := range definition.Actions {
			if action.Collection {
				table.CollectionActions = append(table.CollectionActions, action)
			} else {
				table.RowActions = append(table.RowActions, action)
			}
		}
		model.Tables[definition.Name] = table

		if definition.MutationMode == resource.MutationModeCRUD {
			form := Form{
				Resource:   definition.Name,
				Method:     "post",
				Action:     "/admin/" + definition.Name,
				Permission: definition.PermissionNS + ".create",
			}
			for _, field := range definition.Fields {
				if !field.ReadOnly {
					form.Fields = append(form.Fields, field)
				}
			}
			model.Forms[definition.Name] = form
		}
	}
	sort.Slice(model.Menu, func(i, j int) bool {
		if model.Menu[i].Module == model.Menu[j].Module {
			return model.Menu[i].Label < model.Menu[j].Label
		}
		return model.Menu[i].Module < model.Menu[j].Module
	})
	if len(model.Menu) != len(resources) {
		return Model{}, fmt.Errorf("admin menu generation lost resources")
	}
	return model, nil
}
