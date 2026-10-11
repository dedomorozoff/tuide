package input

import (
	tea "charm.land/bubbletea/v2"
)

// Mapping ties an action to a key combination.
type Mapping struct {
	Action Action
	Mods   tea.KeyMod
	Key    rune
}

// Keymap resolves key presses into actions. Order matters: the first matching
// mapping wins, so user overrides can be prepended.
type Keymap struct {
	mappings []Mapping
}

// New builds a lookup table. Mappings are tried in the given order.
func New(mappings ...Mapping) *Keymap {
	return &Keymap{mappings: mappings}
}

// Bind puts a mapping at the front of the lookup order.
func (k *Keymap) Bind(m Mapping) {
	k.mappings = append([]Mapping{m}, k.mappings...)
}

// Override prepends a mapping, replacing any existing one for the same key.
func (k *Keymap) Override(m Mapping) {
	k.Bind(m)
}

// Lookup returns the action a key press triggers.
func (k *Keymap) Lookup(msg tea.KeyPressMsg) (Action, bool) {
	key := msg.Key()
	for _, mapping := range k.mappings {
		if (Binding{Mods: mapping.Mods, Key: mapping.Key}).Matches(key) {
			return mapping.Action, true
		}
	}
	return "", false
}
