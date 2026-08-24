package cli

import "github.com/LukeSantossz/my-framework/internal/activate"

// HooksStatusPathForTest exposes the hooks path so a test can assert that a
// command which promises to change nothing changed nothing.
func HooksStatusPathForTest(root string) string {
	return activate.HooksStatus(root).Path
}
