package keys

import "charm.land/bubbles/v2/key"

var WorktreeKeys = struct {
	SwitchView key.Binding
}{
	SwitchView: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "switch to notifications"),
	),
}
