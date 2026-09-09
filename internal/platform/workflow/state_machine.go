package workflow

import (
	"errors"
	"fmt"
	"sort"
)

var (
	ErrInvalidState      = errors.New("invalid state")
	ErrInvalidTransition = errors.New("invalid transition")
)

type Transition struct {
	Name       string
	From       string
	To         string
	Permission string
	Command    string
}

type Definition struct {
	Name        string
	Initial     string
	States      []string
	Transitions []Transition
}

func (d Definition) Validate() error {
	if d.Name == "" {
		return errors.New("workflow name is required")
	}
	if d.Initial == "" {
		return errors.New("workflow initial state is required")
	}

	states := make(map[string]struct{}, len(d.States))
	for _, state := range d.States {
		if state == "" {
			return errors.New("workflow state cannot be empty")
		}
		if _, exists := states[state]; exists {
			return fmt.Errorf("duplicate workflow state: %s", state)
		}
		states[state] = struct{}{}
	}
	if _, ok := states[d.Initial]; !ok {
		return fmt.Errorf("%w: initial state %s", ErrInvalidState, d.Initial)
	}

	seenTransitions := make(map[string]struct{}, len(d.Transitions))
	for _, transition := range d.Transitions {
		if transition.Name == "" {
			return errors.New("workflow transition name is required")
		}
		key := transition.From + "\x00" + transition.Name
		if _, exists := seenTransitions[key]; exists {
			return fmt.Errorf("duplicate transition %s from %s", transition.Name, transition.From)
		}
		seenTransitions[key] = struct{}{}
		if _, ok := states[transition.From]; !ok {
			return fmt.Errorf("%w: transition %s from %s", ErrInvalidState, transition.Name, transition.From)
		}
		if _, ok := states[transition.To]; !ok {
			return fmt.Errorf("%w: transition %s to %s", ErrInvalidState, transition.Name, transition.To)
		}
	}
	return nil
}

func (d Definition) Allowed(state string) []Transition {
	allowed := make([]Transition, 0)
	for _, transition := range d.Transitions {
		if transition.From == state {
			allowed = append(allowed, transition)
		}
	}
	sort.Slice(allowed, func(i, j int) bool {
		return allowed[i].Name < allowed[j].Name
	})
	return allowed
}

func (d Definition) Apply(state, transitionName string) (string, error) {
	for _, transition := range d.Transitions {
		if transition.From == state && transition.Name == transitionName {
			return transition.To, nil
		}
	}
	return state, fmt.Errorf("%w: %s from %s", ErrInvalidTransition, transitionName, state)
}
