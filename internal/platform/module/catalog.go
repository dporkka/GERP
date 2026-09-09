package module

// StaticComponent is a descriptor-only component used by the composition root
// before runtime adapters are attached.
type StaticComponent struct {
	Meta Descriptor
}

func (c StaticComponent) Descriptor() Descriptor { return c.Meta }

// DefaultComponents defines Gerp's domain dependency graph. It is deliberately
// coarse-grained: modules communicate through application contracts and shared
// transaction boundaries rather than network calls.
func DefaultComponents() []Component {
	return []Component{
		StaticComponent{Meta: Descriptor{Name: "core", Version: "v0", Description: "tenancy, identity, RBAC, audit and outbox"}},
		StaticComponent{Meta: Descriptor{Name: "finance", Version: "v0", Description: "general ledger and accounting", DependsOn: []string{"core"}}},
		StaticComponent{Meta: Descriptor{Name: "crm", Version: "v0", Description: "accounts, contacts, pipelines and activities", DependsOn: []string{"core"}}},
		StaticComponent{Meta: Descriptor{Name: "inventory", Version: "v0", Description: "items, warehouses and stock ledger", DependsOn: []string{"core"}}},
		StaticComponent{Meta: Descriptor{Name: "compliance", Version: "v0", Description: "business document and tax compliance", DependsOn: []string{"core"}}},
		StaticComponent{Meta: Descriptor{Name: "sales", Version: "v0", Description: "quotes, orders and fulfillment", DependsOn: []string{"crm", "finance", "inventory"}}},
		StaticComponent{Meta: Descriptor{Name: "invoicing", Version: "v0", Description: "invoice lifecycle and compliant documents", DependsOn: []string{"compliance", "finance", "sales"}}},
		StaticComponent{Meta: Descriptor{Name: "procurement", Version: "v0", Description: "requisitions, purchase orders and receipts", DependsOn: []string{"finance", "inventory"}}},
		StaticComponent{Meta: Descriptor{Name: "projects", Version: "v0", Description: "projects, work, time and costing", DependsOn: []string{"core", "finance"}}},
		StaticComponent{Meta: Descriptor{Name: "assets", Version: "v0", Description: "fixed assets and maintenance", DependsOn: []string{"finance", "inventory"}}},
		StaticComponent{Meta: Descriptor{Name: "hcm", Version: "v0", Description: "people, employment and payroll", DependsOn: []string{"core", "finance"}}},
	}
}

func DefaultRegistry() (*Registry, error) {
	registry := NewRegistry()
	for _, component := range DefaultComponents() {
		if err := registry.Register(component); err != nil {
			return nil, err
		}
	}
	if _, err := registry.Resolve(); err != nil {
		return nil, err
	}
	return registry, nil
}
