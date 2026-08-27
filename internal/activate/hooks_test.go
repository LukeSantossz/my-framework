package activate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	framework "github.com/LukeSantossz/my-framework"
)

// The hooks are the primary enforcement point of every gate this framework
// defines, and until these tests existed nothing exercised their logic. What is
// asserted here is behaviour, not text: a failure path that exits 0 by some
// spelling a grep does not know reads as clean, and that is precisely the
// regression docs/specs/0020 and 0027 were written to close.
//
// Each hook is executed as a process, against the bytes the binary ships rather
// than the ones the working tree happens to hold, with a stub `mf` and a stub
// `git` whose answers the test decides. Stubbing both is what makes the
// assertions exact: the test can tell "the gate ran and failed" from "the gate
// could not run", which is the distinction the fail-closed rule is about.

// hookNames are the shipped hooks, and the discovery assertions run over both:
// the two resolve the runner identically on purpose, and the comment in
// commit-msg saying so is not a test.
var hookNames = []string{"commit-msg", "pre-push"}

func bashOrSkip(t *testing.T) string {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not on PATH; the hooks are shell scripts, so there is nothing here to execute")
	}
	return bash
}

// hookRepo lays the shipped hooks out in a temporary directory.
//
// The bytes come from the embedded filesystem rather than from the working
// tree, because what an adopter receives is what has to behave: `mf init` hands
// them these bytes, and a test reading .githooks/ directly would keep passing
// if the embed ever stopped carrying them.
func hookRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, HooksDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(framework.Hooks, framework.HooksPrefix)
	if err != nil {
		t.Fatalf("reading the embedded hooks: %v", err)
	}
	for _, entry := range entries {
		body, err := fs.ReadFile(framework.Hooks, framework.HooksPrefix+"/"+entry.Name())
		if err != nil {
			t.Fatalf("reading the embedded %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), body, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// writeStub puts a stand-in `mf` at path, announcing which of the discovery
// paths it was reached through.
//
// The announcement is the whole point. Asserting only that a gate failed cannot
// distinguish a runner that ran and reported a problem from one the hook never
// found, and those are the two outcomes this suite exists to keep apart.
func writeStub(t *testing.T, path, id string) {
	t.Helper()
	body := fmt.Sprintf(`#!/bin/sh
printf 'ran:%s:%%s\n' "$*"
case "$1" in
  check) exit "${MF_STUB_CHECK_EXIT:-0}" ;;
  review) exit "${MF_STUB_REVIEW_EXIT:-0}" ;;
esac
exit 0
`, id)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// writeGitStub supplies the one git answer the hooks ask for.
//
// Stubbing it is more reliable than looking for a directory outside every
// repository on the machine: the "no repository" case is then a decision this
// test makes rather than a property of wherever the temporary directory landed.
func writeGitStub(t *testing.T, dir string, insideARepository bool) {
	t.Helper()
	body := `#!/bin/sh
case "$1 $2" in
  "rev-parse --show-toplevel") printf '%s\n' "$MF_TEST_REPO_ROOT"; exit 0 ;;
esac
exit 0
`
	if !insideARepository {
		body = `#!/bin/sh
exit 1
`
	}
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// notExecutable writes a file every platform's `[ -x ]` refuses.
//
// Neither a shebang nor an executable extension: the Windows shell decides
// executability by reading the file rather than by its mode bits, so a mode
// alone would not produce the case MF_BIN's guard is about.
func notExecutable(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "not-a-binary.txt")
	if err := os.WriteFile(path, []byte("this is not a program\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// hookEnv builds the environment a hook runs in, from nothing.
//
// PATH holds only what the test put there, so what the hook discovers is
// decided by the test and never by the machine running it — a developer with a
// real `mf` installed would otherwise pass the "no runner" case.
func hookEnv(root string, dirs []string, kv ...string) []string {
	env := []string{
		"PATH=" + strings.Join(dirs, string(os.PathListSeparator)),
		// The git stub answers with this, in the form real git answers in.
		"MF_TEST_REPO_ROOT=" + filepath.ToSlash(root),
	}
	// Carried through rather than invented: a shell started with no SYSTEMROOT
	// fails on Windows for reasons that have nothing to do with these hooks.
	for _, name := range []string{"SYSTEMROOT", "SystemRoot", "TEMP", "TMP", "HOME"} {
		if v := os.Getenv(name); v != "" {
			env = append(env, name+"="+v)
		}
	}
	return append(env, kv...)
}

// runHook executes one hook the way git does: as a process, in the repository,
// with only what its environment gives it.
func runHook(t *testing.T, root, hook string, env []string, args ...string) (int, string) {
	t.Helper()
	bash := bashOrSkip(t)
	cmd := exec.Command(bash, append([]string{filepath.Join(root, HooksDir, hook)}, args...)...)
	cmd.Dir = root
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), string(out)
	}
	t.Fatalf("running %s: %v\n%s", hook, err, out)
	return 0, ""
}

// hookArgs is what git hands each hook. commit-msg is given the file holding
// the message being written; pre-push is given the remote and its URL, and
// reads neither.
func hookArgs(t *testing.T, hook, root string) []string {
	t.Helper()
	if hook != "commit-msg" {
		return []string{"origin", "https://example.invalid/repo.git"}
	}
	path := filepath.Join(root, "COMMIT_EDITMSG")
	if err := os.WriteFile(path, []byte("feat: a subject the vocabulary accepts\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return []string{path}
}

// --- runner discovery ---------------------------------------------------------

func TestHookPrefersMFBinOverEverythingElse(t *testing.T) {
	for _, hook := range hookNames {
		t.Run(hook, func(t *testing.T) {
			root := hookRepo(t)
			binDir := t.TempDir()
			writeGitStub(t, binDir, true)
			writeStub(t, filepath.Join(binDir, "mf"), "path")
			writeStub(t, filepath.Join(root, "mf"), "root")
			override := filepath.Join(t.TempDir(), "mf")
			writeStub(t, override, "mfbin")

			code, out := runHook(t, root, hook,
				hookEnv(root, []string{binDir}, "MF_BIN="+override),
				hookArgs(t, hook, root)...)
			if code != 0 {
				t.Fatalf("exit %d:\n%s", code, out)
			}
			if !strings.Contains(out, "ran:mfbin:") {
				t.Errorf("MF_BIN was not the runner:\n%s", out)
			}
			for _, other := range []string{"ran:path:", "ran:root:"} {
				if strings.Contains(out, other) {
					t.Errorf("%s ran as well as MF_BIN:\n%s", other, out)
				}
			}
		})
	}
}

func TestHookRefusesANonExecutableMFBin(t *testing.T) {
	// An override that points at nothing must not be read as no override: the
	// fallback would then run a different binary than the one the person named,
	// and say nothing about it.
	for _, hook := range hookNames {
		t.Run(hook, func(t *testing.T) {
			root := hookRepo(t)
			binDir := t.TempDir()
			writeGitStub(t, binDir, true)
			writeStub(t, filepath.Join(binDir, "mf"), "path")

			code, out := runHook(t, root, hook,
				hookEnv(root, []string{binDir}, "MF_BIN="+notExecutable(t, t.TempDir())),
				hookArgs(t, hook, root)...)
			if code == 0 {
				t.Fatalf("a broken MF_BIN passed the gate:\n%s", out)
			}
			if !strings.Contains(out, "MF_BIN") {
				t.Errorf("the refusal does not name MF_BIN:\n%s", out)
			}
			if strings.Contains(out, "ran:") {
				t.Errorf("the hook fell through to another runner:\n%s", out)
			}
		})
	}
}

func TestHookFallsBackThroughPathThenTheRepositoryRoot(t *testing.T) {
	// The order is the contract: a binary on PATH is the installed one, and the
	// two at the repository root are what a checkout builds for itself. This is
	// how CI runs the gates on this repository.
	for _, hook := range hookNames {
		for _, tc := range []struct {
			name string
			id   string
			// place puts the only stub where this case expects it to be found.
			place func(t *testing.T, root, binDir string)
		}{
			{"on PATH", "path", func(t *testing.T, root, binDir string) {
				writeStub(t, filepath.Join(binDir, "mf"), "path")
			}},
			{"at the repository root", "root", func(t *testing.T, root, binDir string) {
				writeStub(t, filepath.Join(root, "mf"), "root")
			}},
			{"at the repository root as mf.exe", "rootexe", func(t *testing.T, root, binDir string) {
				writeStub(t, filepath.Join(root, "mf.exe"), "rootexe")
			}},
		} {
			t.Run(hook+"/"+tc.name, func(t *testing.T) {
				root := hookRepo(t)
				binDir := t.TempDir()
				writeGitStub(t, binDir, true)
				tc.place(t, root, binDir)

				code, out := runHook(t, root, hook,
					hookEnv(root, []string{binDir}),
					hookArgs(t, hook, root)...)
				if code != 0 {
					t.Fatalf("exit %d:\n%s", code, out)
				}
				if want := "ran:" + tc.id + ":"; !strings.Contains(out, want) {
					t.Errorf("the runner %s was not the one used:\n%s", tc.name, out)
				}
			})
		}
	}
}

func TestHookFailsClosedWhenNoRunnerIsFound(t *testing.T) {
	// A gate that cannot find its runner has not passed; it has not run, and
	// those are only the same thing to a hook that lies.
	for _, hook := range hookNames {
		t.Run(hook, func(t *testing.T) {
			root := hookRepo(t)
			binDir := t.TempDir()
			writeGitStub(t, binDir, true)

			code, out := runHook(t, root, hook,
				hookEnv(root, []string{binDir}),
				hookArgs(t, hook, root)...)
			if code == 0 {
				t.Fatalf("the hook passed with no runner at all:\n%s", out)
			}
			if !strings.Contains(out, "mf") {
				t.Errorf("the failure does not say what is missing:\n%s", out)
			}
		})
	}
}

func TestHookFailsClosedOutsideARepository(t *testing.T) {
	// Every gate is a question about a repository, so failing to identify one
	// means no question can be asked. That is not a pass.
	for _, hook := range hookNames {
		t.Run(hook, func(t *testing.T) {
			root := hookRepo(t)
			binDir := t.TempDir()
			writeGitStub(t, binDir, false)
			writeStub(t, filepath.Join(binDir, "mf"), "path")

			code, out := runHook(t, root, hook,
				hookEnv(root, []string{binDir}),
				hookArgs(t, hook, root)...)
			if code == 0 {
				t.Fatalf("the hook passed with no repository to gate:\n%s", out)
			}
			if strings.Contains(out, "ran:") {
				t.Errorf("a gate ran against a repository that could not be identified:\n%s", out)
			}
		})
	}
}

// --- commit-msg ---------------------------------------------------------------

func TestCommitMsgChecksTheMessageFileGitHandedIt(t *testing.T) {
	// The subject under the author's cursor, in the commit they can still
	// amend — not the commits already on the branch, which would report the
	// right violation one commit too late to be the answer to it.
	root := hookRepo(t)
	binDir := t.TempDir()
	writeGitStub(t, binDir, true)
	writeStub(t, filepath.Join(binDir, "mf"), "path")
	args := hookArgs(t, "commit-msg", root)

	code, out := runHook(t, root, "commit-msg", hookEnv(root, []string{binDir}), args...)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	want := "ran:path:check commit --message " + args[0]
	if !strings.Contains(out, want) {
		t.Errorf("the runner was not called as %q:\n%s", want, out)
	}
}

func TestCommitMsgStopsTheCommitWhenTheCheckFails(t *testing.T) {
	root := hookRepo(t)
	binDir := t.TempDir()
	writeGitStub(t, binDir, true)
	writeStub(t, filepath.Join(binDir, "mf"), "path")

	code, out := runHook(t, root, "commit-msg",
		hookEnv(root, []string{binDir}, "MF_STUB_CHECK_EXIT=1"),
		hookArgs(t, "commit-msg", root)...)
	if code == 0 {
		t.Fatalf("a rejected message did not stop the commit:\n%s", out)
	}
	if !strings.Contains(out, "--no-verify") {
		t.Errorf("the refusal does not name the bypass:\n%s", out)
	}
}

// --- pre-push -----------------------------------------------------------------

func TestPrePushStopsThePushWhenTheChecksFail(t *testing.T) {
	root := hookRepo(t)
	binDir := t.TempDir()
	writeGitStub(t, binDir, true)
	writeStub(t, filepath.Join(binDir, "mf"), "path")

	code, out := runHook(t, root, "pre-push",
		hookEnv(root, []string{binDir}, "MF_STUB_CHECK_EXIT=1"),
		hookArgs(t, "pre-push", root)...)
	if code == 0 {
		t.Fatalf("failing checks did not stop the push:\n%s", out)
	}
	if strings.Contains(out, "review") {
		t.Errorf("the review ran after the checks had already failed:\n%s", out)
	}
}

func TestPrePushLetsAReviewThatOnlyReportsFindingsThrough(t *testing.T) {
	// Every review layer is advisory unless roles.r2.blocking is on, and that
	// decision belongs to the configuration the review reads — not to the hook,
	// which only propagates what it is told.
	root := hookRepo(t)
	binDir := t.TempDir()
	writeGitStub(t, binDir, true)
	writeStub(t, filepath.Join(binDir, "mf"), "path")

	code, out := runHook(t, root, "pre-push",
		hookEnv(root, []string{binDir}),
		hookArgs(t, "pre-push", root)...)
	if code != 0 {
		t.Fatalf("exit %d with both gates passing:\n%s", code, out)
	}
	for _, want := range []string{"ran:path:check\n", "ran:path:review --role r2"} {
		if !strings.Contains(out, want) {
			t.Errorf("the hook did not run %q:\n%s", want, out)
		}
	}
}

func TestPrePushPropagatesABlockingReview(t *testing.T) {
	root := hookRepo(t)
	binDir := t.TempDir()
	writeGitStub(t, binDir, true)
	writeStub(t, filepath.Join(binDir, "mf"), "path")

	code, out := runHook(t, root, "pre-push",
		hookEnv(root, []string{binDir}, "MF_STUB_REVIEW_EXIT=3"),
		hookArgs(t, "pre-push", root)...)
	if code != 3 {
		t.Fatalf("exit %d, want the review's own 3:\n%s", code, out)
	}
}

func TestSkipR2ReviewSkipsTheReviewAndNotTheChecks(t *testing.T) {
	// The checks call no model and cost nothing; the review can take minutes.
	// A skip that took the free gates with it would be a reason to set the
	// variable permanently.
	root := hookRepo(t)
	binDir := t.TempDir()
	writeGitStub(t, binDir, true)
	writeStub(t, filepath.Join(binDir, "mf"), "path")

	code, out := runHook(t, root, "pre-push",
		hookEnv(root, []string{binDir}, "SKIP_R2_REVIEW=1"),
		hookArgs(t, "pre-push", root)...)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "ran:path:check\n") {
		t.Errorf("the checks were skipped along with the review:\n%s", out)
	}
	if strings.Contains(out, "review") {
		t.Errorf("the review ran despite SKIP_R2_REVIEW=1:\n%s", out)
	}
}
