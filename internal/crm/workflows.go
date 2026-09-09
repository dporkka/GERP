package crm

import "gerp/internal/platform/workflow"

var DealLifecycle = workflow.Definition{
	Name:    "crm.deal.lifecycle",
	Initial: "open",
	States:  []string{"open", "won", "lost"},
	Transitions: []workflow.Transition{
		{Name: "win", From: "open", To: "won", Permission: "crm.deal.win", Command: "crm.win_deal"},
		{Name: "lose", From: "open", To: "lost", Permission: "crm.deal.lose", Command: "crm.lose_deal"},
		{Name: "reopen", From: "lost", To: "open", Permission: "crm.deal.reopen", Command: "crm.reopen_deal"},
	},
}

// ValidateWorkflows fails application startup if a built-in CRM state machine is
// internally inconsistent. Archive/restore is intentionally orthogonal to deal
// outcome so history can distinguish record retention from sales lifecycle.
func ValidateWorkflows() error {
	return DealLifecycle.Validate()
}
