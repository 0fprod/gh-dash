package prview

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func TestPRView_CtrlRRequestsRebase(t *testing.T) {
	action := MsgToAction(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	require.NotNil(t, action)
	require.Equal(t, PRActionRebase, action.Type)
}
