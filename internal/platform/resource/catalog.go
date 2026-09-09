package resource

func CoreResources() []Resource {
	return []Resource{
		{
			Name:         "contacts",
			Label:        "Contact",
			PluralLabel:  "Contacts",
			Module:       "crm",
			PermissionNS: "crm.contact",
			MutationMode: MutationModeCRUD,
			TenantScoped: true,
			Audited:      true,
			Fields: []Field{
				{Name: "display_name", Label: "Name", Kind: FieldString, Required: true, Searchable: true, Sortable: true},
				{Name: "email", Label: "Email", Kind: FieldString, Searchable: true, Filterable: true},
				{Name: "phone", Label: "Phone", Kind: FieldString, Searchable: true},
				{Name: "owner_id", Label: "Owner", Kind: FieldUUID, Filterable: true},
				{Name: "lifecycle_stage", Label: "Lifecycle", Kind: FieldEnum, Filterable: true, Options: []string{"lead", "prospect", "customer", "inactive"}},
			},
			DefaultSort:   "display_name",
			DefaultSearch: []string{"display_name", "email", "phone"},
		},
		{
			Name:         "deals",
			Label:        "Deal",
			PluralLabel:  "Deals",
			Module:       "crm",
			PermissionNS: "crm.deal",
			MutationMode: MutationModeCommandsOnly,
			TenantScoped: true,
			Audited:      true,
			Fields: []Field{
				{Name: "name", Label: "Name", Kind: FieldString, Required: true, Searchable: true, Sortable: true},
				{Name: "amount_minor", Label: "Amount", Kind: FieldMoney, Sortable: true},
				{Name: "stage_id", Label: "Stage", Kind: FieldUUID, Filterable: true},
				{Name: "owner_id", Label: "Owner", Kind: FieldUUID, Filterable: true},
				{Name: "status", Label: "Status", Kind: FieldEnum, Filterable: true, ReadOnly: true, Options: []string{"open", "won", "lost", "archived"}},
			},
			Actions: []Action{
				{Name: "move", Label: "Move stage", Permission: "crm.deal.move", Command: "crm.move_deal"},
				{Name: "win", Label: "Mark won", Permission: "crm.deal.win", Command: "crm.win_deal"},
				{Name: "lose", Label: "Mark lost", Permission: "crm.deal.lose", Command: "crm.lose_deal"},
			},
			DefaultSort:   "updated_at",
			DefaultSearch: []string{"name"},
		},
		{
			Name:         "journal_entries",
			Label:        "Journal entry",
			PluralLabel:  "Journal entries",
			Module:       "finance",
			PermissionNS: "finance.journal",
			MutationMode: MutationModeCommandsOnly,
			TenantScoped: true,
			Audited:      true,
			Fields: []Field{
				{Name: "number", Label: "Number", Kind: FieldString, Required: true, Searchable: true, Sortable: true, ReadOnly: true},
				{Name: "effective_date", Label: "Date", Kind: FieldDate, Required: true, Sortable: true, Filterable: true, ReadOnly: true},
				{Name: "currency", Label: "Currency", Kind: FieldString, Required: true, Filterable: true, ReadOnly: true},
				{Name: "description", Label: "Description", Kind: FieldText, Searchable: true, ReadOnly: true},
			},
			Actions: []Action{
				{Name: "post", Label: "Post journal", Permission: "finance.journal.post", Command: "finance.post_journal"},
				{Name: "reverse", Label: "Reverse journal", Permission: "finance.journal.reverse", Command: "finance.reverse_journal", Dangerous: true},
			},
			DefaultSort:   "effective_date",
			DefaultSearch: []string{"number", "description"},
		},
	}
}
