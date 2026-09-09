package sales

import "gerp/internal/platform/workflow"

func QuoteWorkflow() workflow.Definition {
	return workflow.Definition{
		Name:    "sales.quote",
		Initial: "draft",
		States:  []string{"draft", "sent", "accepted", "rejected", "expired"},
		Transitions: []workflow.Transition{
			{Name: "send", From: "draft", To: "sent", Permission: "sales.quote.send", Command: "sales.send_quote"},
			{Name: "accept", From: "sent", To: "accepted", Permission: "sales.quote.accept", Command: "sales.accept_quote"},
			{Name: "reject", From: "sent", To: "rejected", Permission: "sales.quote.reject", Command: "sales.reject_quote"},
			{Name: "expire", From: "sent", To: "expired", Permission: "sales.quote.expire", Command: "sales.expire_quote"},
		},
	}
}

func OrderWorkflow() workflow.Definition {
	return workflow.Definition{
		Name:    "sales.order",
		Initial: "confirmed",
		States:  []string{"confirmed", "fulfilled", "cancelled"},
		Transitions: []workflow.Transition{
			{Name: "fulfill", From: "confirmed", To: "fulfilled", Permission: "sales.order.fulfill", Command: "sales.fulfill_order"},
			{Name: "cancel", From: "confirmed", To: "cancelled", Permission: "sales.order.cancel", Command: "sales.cancel_order"},
		},
	}
}

func InvoiceWorkflow() workflow.Definition {
	return workflow.Definition{
		Name:    "sales.invoice",
		Initial: "draft",
		States:  []string{"draft", "issued", "paid", "void"},
		Transitions: []workflow.Transition{
			{Name: "issue", From: "draft", To: "issued", Permission: "sales.invoice.issue", Command: "sales.issue_invoice"},
			{Name: "mark_paid", From: "issued", To: "paid", Permission: "sales.invoice.mark_paid", Command: "sales.mark_invoice_paid"},
			{Name: "void", From: "issued", To: "void", Permission: "sales.invoice.void", Command: "sales.void_invoice"},
		},
	}
}
