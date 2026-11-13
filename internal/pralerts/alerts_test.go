package pralerts_test

import (
	"testing"

	"github.com/dlvhdr/gh-dash/v4/internal/pralerts"
	"github.com/stretchr/testify/require"
)

func TestPRAlerts_NewEventsAndFailureAreReportedOnlyOnce(t *testing.T) {
	path := t.TempDir() + "/state.json"
	first := []pralerts.PR{{ID: "PR1", Repo: "owner/repo", Number: 42, Title: "Example", CI: "SUCCESS"}}
	messages, err := pralerts.Process(path, first)
	require.NoError(t, err)
	require.Empty(t, messages)

	first[0].CI = "FAILURE"
	first[0].Events = []pralerts.Event{{ID: "c1", Kind: "Comentario", Actor: "alice"}, {ID: "r1", Kind: "Aprobación", Actor: "bob"}}
	messages, err = pralerts.Process(path, first)
	require.NoError(t, err)
	require.Len(t, messages, 3)
	messages, err = pralerts.Process(path, first)
	require.NoError(t, err)
	require.Empty(t, messages)
}
