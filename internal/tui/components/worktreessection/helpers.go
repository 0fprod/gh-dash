package worktreessection

import "os"

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }
func homeDir() (string, error) {
	return os.UserHomeDir()
}
