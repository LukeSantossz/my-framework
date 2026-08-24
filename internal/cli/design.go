package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LukeSantossz/my-framework/internal/config"
	"github.com/LukeSantossz/my-framework/internal/design"
)

// designGate checks the rendered surfaces against docs/standards/design.md.
//
// It lives here rather than among the check package's gates for the same reason
// the instruction-file gate does: the list of surfaces is this project's
// application of the standard, so it comes from configuration, while the
// vocabulary itself comes from the standard that owns it.
//
// No model, no network: the whole gate is a parse and a set membership test.
func designGate(env Env, cfg *config.Config) int {
	palette, err := design.Load(env.RepoRoot)
	if err != nil {
		// A standard that cannot be parsed stops the gate. Continuing would
		// leave an empty palette accepting every colour, which is a gate
		// reporting success exactly when it stopped checking.
		fmt.Fprintf(env.Stdout, "FAIL %-8s %v\n", "design", err)
		return 1
	}

	violations := append(palette.CheckNeutrality(), palette.CheckDerivation()...)

	surfaces := designSurfaces(cfg)
	checked := 0
	for _, rel := range surfaces {
		body, readErr := os.ReadFile(filepath.Join(env.RepoRoot, filepath.FromSlash(rel)))
		if readErr != nil {
			violations = append(violations, design.Violation{
				File:   rel,
				Value:  "unreadable",
				Reason: "declared as a design surface but cannot be read: " + readErr.Error(),
			})
			continue
		}
		checked++
		violations = append(violations, palette.CheckSurface(rel, string(body))...)
	}

	if len(violations) > 0 {
		fmt.Fprintf(env.Stdout, "FAIL %-8s %d problem(s)\n", "design", len(violations))
		for _, v := range violations {
			fmt.Fprintf(env.Stdout, "       %s\n", v)
		}
		return 1
	}

	// What was checked is reported, including when the answer is "nothing".
	// A gate that says `ok` while checking no surfaces is indistinguishable
	// from one that checked them and found them clean.
	if checked == 0 {
		fmt.Fprintf(env.Stdout, "ok   %-8s %d colour role(s) declared; no surfaces declared to check\n",
			"design", len(palette.Colors))
		return 0
	}
	fmt.Fprintf(env.Stdout, "ok   %-8s %d surface(s) against %d declared colour role(s)\n",
		"design", checked, len(palette.Colors))
	return 0
}

// designSurfaces is what this project renders. It is project policy rather than
// a standard: an adopter's surfaces are their own files.
func designSurfaces(cfg *config.Config) []string {
	if cfg == nil || cfg.Project == nil {
		return nil
	}
	return cfg.Project.Checks.DesignSurfaces
}
