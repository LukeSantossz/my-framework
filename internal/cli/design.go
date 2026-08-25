package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/LukeSantossz/my-framework/internal/config"
	"github.com/LukeSantossz/my-framework/internal/design"
)

// designGate checks the rendered surfaces against the design.md this
// repository keeps with its standards.
//
// It lives here rather than among the check package's gates for the same reason
// the instruction-file gate does: the list of surfaces is this project's
// application of the standard, so it comes from configuration, while the
// vocabulary itself comes from the standard that owns it.
//
// No model, no network: the whole gate is a parse and a set membership test.
func designGate(env Env, cfg *config.Config) int {
	surfaces := designSurfaces(cfg)

	palette, err := design.Load(env.RepoRoot, standardsDir(cfg))
	if err != nil {
		// An absent standard is the absence of this gate's own input, not a
		// failure of the repository. An adopter whose vendored corpus carries
		// no design.md could otherwise not run `mf check` at all — the same
		// adoption defect docs/specs/0027 fixed for the standards directory,
		// left in place for one file inside it.
		//
		// It is only absence when nothing is declared. A repository that names
		// design surfaces has said it renders something the identity governs,
		// so a missing standard there is a contradiction rather than a gate
		// that was never adopted, and docs/adr/0011 calls this gate binding.
		if errors.Is(err, fs.ErrNotExist) && len(surfaces) == 0 {
			fmt.Fprintf(env.Stdout, "ok   %-8s no %s here; nothing declares a surface it governs\n",
				"design", filepath.ToSlash(filepath.Join(standardsDir(cfg), design.StandardFileName)))
			return 0
		}
		// A standard that cannot be parsed stops the gate. Continuing would
		// leave an empty palette accepting every colour, which is a gate
		// reporting success exactly when it stopped checking.
		fmt.Fprintf(env.Stdout, "FAIL %-8s %v\n", "design", err)
		return 1
	}

	violations := append(palette.CheckNeutrality(), palette.CheckDerivation()...)

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
