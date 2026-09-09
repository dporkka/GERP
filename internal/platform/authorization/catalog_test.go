package authorization

import (
	"testing"

	"gerp/internal/platform/resource"
)

func TestFromResourcesDoesNotGenerateCRUDForCommandsOnly(t *testing.T) {
	permissions, err := FromResources(resource.CoreResources())
	if err != nil {
		t.Fatalf("generate permissions: %v", err)
	}

	codes := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		codes[permission.Code] = struct{}{}
	}

	for _, expected := range []string{
		"crm.contact.read",
		"crm.contact.create",
		"crm.contact.update",
		"crm.deal.read",
		"crm.deal.win",
		"finance.journal.read",
		"finance.journal.post",
		"finance.journal.reverse",
	} {
		if _, ok := codes[expected]; !ok {
			t.Errorf("missing permission %s", expected)
		}
	}

	for _, forbidden := range []string{
		"crm.deal.update",
		"finance.journal.update",
		"finance.journal.create",
	} {
		if _, ok := codes[forbidden]; ok {
			t.Errorf("commands-only resource unexpectedly generated %s", forbidden)
		}
	}
}
