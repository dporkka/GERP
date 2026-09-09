package admin

import (
	"context"
	"fmt"
	"html"
	"io"
	"sort"
	"strings"

	"gerp/internal/platform/resource"

	"github.com/a-h/templ"
)

type Renderer struct{}

func NewRenderer() Renderer { return Renderer{} }

func (Renderer) Navigation(model Model) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		if _, err := io.WriteString(w, `<nav aria-label="ERP modules"><ul>`); err != nil {
			return err
		}
		for _, item := range model.Menu {
			if _, err := fmt.Fprintf(w,
				`<li data-module="%s" data-permission="%s"><a href="%s" hx-get="%s" hx-target="#resource-panel" hx-push-url="true">%s</a></li>`,
				escape(item.Module), escape(item.Permission), escape(item.Path), escape(item.Path), escape(item.Label)); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, `</ul></nav>`)
		return err
	})
}

func (Renderer) ResourceForm(definition resource.Resource, values map[string]string, validationErrors map[string]string) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		if definition.MutationMode != resource.MutationModeCRUD {
			return fmt.Errorf("resource %s does not permit generic forms", definition.Name)
		}
		path := "/admin/" + definition.Name
		if _, err := fmt.Fprintf(w, `<form method="post" action="%s" hx-post="%s" hx-target="#resource-panel" hx-swap="outerHTML">`, escape(path), escape(path)); err != nil {
			return err
		}
		for _, field := range definition.Fields {
			if field.ReadOnly {
				continue
			}
			if _, err := fmt.Fprintf(w, `<label for="%s">%s</label>`, escape(field.Name), escape(field.Label)); err != nil {
				return err
			}
			if errText := validationErrors[field.Name]; errText != "" {
				if _, err := fmt.Fprintf(w, `<span role="alert">%s</span>`, escape(errText)); err != nil {
					return err
				}
			}
			if err := renderField(w, field, values[field.Name]); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, `<button type="submit">Save</button></form>`)
		return err
	})
}

func (Renderer) ResourceTable(definition resource.Resource, rows []map[string]any) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		if _, err := fmt.Fprintf(w, `<section id="resource-panel" data-resource="%s">`, escape(definition.Name)); err != nil {
			return err
		}
		for _, action := range definition.Actions {
			if !action.Collection {
				continue
			}
			if err := renderAction(w, definition.Name, "", action); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, `<table><thead><tr>`); err != nil {
			return err
		}
		visible := make([]resource.Field, 0, len(definition.Fields))
		for _, field := range definition.Fields {
			if field.Sensitive {
				continue
			}
			visible = append(visible, field)
			if _, err := fmt.Fprintf(w, `<th scope="col">%s</th>`, escape(field.Label)); err != nil {
				return err
			}
		}
		if len(definition.Actions) > 0 {
			if _, err := io.WriteString(w, `<th scope="col">Actions</th>`); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, `</tr></thead><tbody>`); err != nil {
			return err
		}
		for _, row := range rows {
			if _, err := io.WriteString(w, `<tr>`); err != nil {
				return err
			}
			for _, field := range visible {
				if _, err := fmt.Fprintf(w, `<td>%s</td>`, escape(fmt.Sprint(row[field.Name]))); err != nil {
					return err
				}
			}
			if len(definition.Actions) > 0 {
				if _, err := io.WriteString(w, `<td>`); err != nil {
					return err
				}
				id := fmt.Sprint(row["id"])
				for _, action := range definition.Actions {
					if action.Collection {
						continue
					}
					if err := renderAction(w, definition.Name, id, action); err != nil {
						return err
					}
				}
				if _, err := io.WriteString(w, `</td>`); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, `</tr>`); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, `</tbody></table></section>`)
		return err
	})
}

func renderAction(w io.Writer, resourceName, id string, action resource.Action) error {
	path := "/admin/" + resourceName
	if id != "" {
		path += "/" + id
	}
	path += "/actions/" + action.Name
	confirm := ""
	if action.Dangerous {
		confirm = ` hx-confirm="Are you sure?"`
	}
	_, err := fmt.Fprintf(w,
		`<button type="button" hx-post="%s" hx-target="#resource-panel" hx-swap="outerHTML" data-command="%s" data-permission="%s"%s>%s</button>`,
		escape(path), escape(action.Command), escape(action.Permission), confirm, escape(action.Label))
	return err
}

func renderField(w io.Writer, field resource.Field, value string) error {
	required := ""
	if field.Required {
		required = " required"
	}
	name := escape(field.Name)
	value = escape(value)
	switch field.Kind {
	case resource.FieldText, resource.FieldJSON:
		_, err := fmt.Fprintf(w, `<textarea id="%s" name="%s"%s>%s</textarea>`, name, name, required, value)
		return err
	case resource.FieldEnum:
		if _, err := fmt.Fprintf(w, `<select id="%s" name="%s"%s>`, name, name, required); err != nil {
			return err
		}
		for _, option := range field.Options {
			selected := ""
			if option == value {
				selected = " selected"
			}
			if _, err := fmt.Fprintf(w, `<option value="%s"%s>%s</option>`, escape(option), selected, escape(option)); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, `</select>`)
		return err
	case resource.FieldBoolean:
		checked := ""
		if strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "on") {
			checked = " checked"
		}
		_, err := fmt.Fprintf(w, `<input id="%s" name="%s" type="checkbox" value="true"%s%s>`, name, name, checked, required)
		return err
	default:
		inputType := "text"
		switch field.Kind {
		case resource.FieldInteger, resource.FieldMoney, resource.FieldDecimal:
			inputType = "number"
		case resource.FieldDate:
			inputType = "date"
		case resource.FieldDateTime:
			inputType = "datetime-local"
		}
		_, err := fmt.Fprintf(w, `<input id="%s" name="%s" type="%s" value="%s"%s>`, name, name, inputType, value, required)
		return err
	}
}

func PermissionCodes(model Model) []string {
	codes := make([]string, 0, len(model.Permissions))
	for _, permission := range model.Permissions {
		codes = append(codes, permission.Code)
	}
	sort.Strings(codes)
	return codes
}

func escape(value string) string { return html.EscapeString(value) }
