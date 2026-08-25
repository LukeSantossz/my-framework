package check

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// The durability rule in spec_method.md — a number runs from 0001, is never
// reused, and a superseded record is marked Retired in place — is checked in
// three ways, because no one of them sees what the others do.
//
// Contiguity, in check.go, catches a hole. It cannot catch a deletion at the
// end: removing the highest-numbered record leaves 0001..N-1 contiguous and
// clean, which is the exact shape of the incident the rule was written after.
// So the archive is also checked against git history, where absence is still
// visible.
//
// The header check is the third: a directory of files is not an archive unless
// each file says what it is, and a spec that lost its header is a spec nothing
// can find.
//
// These three guards, and the pins below, are what left with
// scripts/test/docs-consistency.test.sh when docs/specs/0027 deleted the shell
// path. `mf check docs` and the numbering above cover the rest of that suite;
// this file is what had no replacement.

// ArchiveMarker precedes the fenced block in which a repository records the two
// facts about its durable archive that the archive itself cannot carry: where
// each backfilled record was extracted from, and which records were removed
// before the rule forbidding removal existed.
//
// It is an HTML comment for the same reason design.Marker is — invisible in a
// rendered document, unambiguous to a parser — and it is looked for in the ADR
// directory rather than among the standards because those facts are a
// decision's record, not a rule. They are true of one repository's history and
// of no other, so a repository that vendors these standards inherits the rule
// and none of the exceptions. Recording nothing means no pins and no accounted
// deletions, which is the strict reading: every deletion then fails.
const ArchiveMarker = "<!-- mf:records archive -->"

// specHeader is what says a document in the archive is a spec.
const specHeader = "# SPEC:"

// archive is what the recorded block declares.
type archive struct {
	// Extracted pins an archived record, named relative to the specs
	// directory, to the git object it was copied from — `<commit>:SPEC.md` for
	// records backfilled out of a working file that each cycle overwrote. The
	// comparison is blob against blob rather than text against text, so a
	// line-ending conversion cannot make a verbatim copy look edited, and an
	// edit cannot hide behind one.
	Extracted map[string]string `toml:"extracted"`

	// Deleted is the closed list of records removed before the durability rule
	// existed, each naming the record that carries its decision today.
	//
	// It is not the retirement mechanism: spec_method.md keeps a retired record
	// in place with its number and its file, marked Retired, so retiring one
	// never needs an entry here. Requiring a surviving record to be named is
	// what stops the list from becoming a way to wave any deletion through by
	// appending a line to it — the failure the guard exists to prevent,
	// reintroduced through the guard's own exception list.
	Deleted map[string]string `toml:"deleted"`

	// Source is the document the block was read from, so a violation names the
	// file a reader has to open.
	Source string `toml:"-"`
}

// loadArchive reads the block out of the repository's decision records.
//
// Two documents carrying it is an error rather than a merge: the block is a
// record of what the archive is made of, and two records of one fact are the
// parallel list docs/adr/0009 exists to forbid.
func loadArchive(o Options) (archive, error) {
	entries, err := os.ReadDir(o.ADRDir)
	if err != nil {
		if os.IsNotExist(err) {
			return archive{}, nil
		}
		return archive{}, err
	}
	var carrying []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		body, err := o.read(o.ADRDir, name)
		if err != nil {
			return archive{}, err
		}
		if strings.Contains(body, ArchiveMarker) {
			carrying = append(carrying, name)
		}
	}
	switch len(carrying) {
	case 0:
		return archive{}, nil
	case 1:
		body, err := o.read(o.ADRDir, carrying[0])
		if err != nil {
			return archive{}, err
		}
		return parseArchive(carrying[0], body)
	default:
		return archive{}, fmt.Errorf("%s appears in %s; one archive is recorded in one place, or the two records drift",
			ArchiveMarker, strings.Join(carrying, " and "))
	}
}

// parseArchive reads the fenced block after the marker. Every failure is an
// error for the reason ParseTypeTable's are: a block that quietly became an
// empty one would turn every pin and every accounted deletion off at once, and
// report success for doing it.
func parseArchive(name, body string) (archive, error) {
	// git hands a Windows checkout CRLF, and the TOML decoder refuses a
	// carriage return as a control character; without this the same document
	// parses on one machine and fails on another.
	body = strings.ReplaceAll(body, "\r\n", "\n")

	at := strings.Index(body, ArchiveMarker)
	if at < 0 {
		return archive{}, fmt.Errorf("%s: no %s marker", name, ArchiveMarker)
	}
	block, ok := firstFencedBlock(body[at+len(ArchiveMarker):])
	if !ok {
		return archive{}, fmt.Errorf("%s: the %s marker is not followed by a fenced block", name, ArchiveMarker)
	}
	var a archive
	if _, err := toml.Decode(block, &a); err != nil {
		return archive{}, fmt.Errorf("%s: the archive block is not valid TOML: %w", name, err)
	}
	a.Source = name
	return a, nil
}

// specHeaders reports every document in the specs directory that does not
// announce itself as a spec.
//
// It globs rather than reading a list, so a spec added after the guard is
// covered by it, and it covers every `.md` rather than only the numbered ones:
// the directory is the archive, so a file in it that is not a spec is either a
// spec that lost its header or something that does not belong there.
func specHeaders(o Options) ([]Problem, error) {
	entries, err := os.ReadDir(o.SpecsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var problems []Problem
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		body, err := o.read(o.SpecsDir, name)
		if err != nil {
			problems = append(problems, Problem{File: "specs/" + name, Message: "cannot read the record: " + err.Error()})
			continue
		}
		if !strings.HasPrefix(firstLine(body), specHeader) {
			problems = append(problems, Problem{File: "specs/" + name,
				Message: fmt.Sprintf("does not open with %q; the archive holds specs, and the header is what says a document is one", specHeader)})
		}
	}
	return problems, nil
}

// firstLine returns the opening line with the two things a Markdown file picks
// up in transit removed: the byte-order mark an editor may write, and the
// carriage return a Windows checkout carries.
func firstLine(body string) string {
	body = strings.TrimPrefix(body, "")
	line, _, _ := strings.Cut(body, "\n")
	return strings.TrimRight(line, "\r")
}

// archivePins checks each backfilled record against the object it was
// extracted from. It proves the archive is verbatim history rather than a
// retelling of it, which is what makes an archived spec usable as evidence of
// what was approved at the time.
func archivePins(o Options, a archive) []Problem {
	if len(a.Extracted) == 0 {
		return nil
	}
	specs := o.specsPrefix()
	var problems []Problem
	for _, name := range sortedKeys(a.Extracted) {
		source := a.Extracted[name]
		archived := "HEAD:" + path.Join(specs, name)
		have, err := o.Repo.ObjectID(archived)
		if err != nil {
			problems = append(problems, Problem{File: "specs/" + name,
				Message: fmt.Sprintf("%s pins this record to the extraction %s, and %s is not committed", a.Source, source, archived)})
			continue
		}
		want, err := o.Repo.ObjectID(source)
		if err != nil {
			problems = append(problems, Problem{File: "specs/" + name,
				Message: fmt.Sprintf("%s pins this record to %s, which does not resolve here; the pin needs the full history it was recorded against", a.Source, source)})
			continue
		}
		if have != want {
			problems = append(problems, Problem{File: "specs/" + name,
				Message: fmt.Sprintf("differs from the extraction %s pins it to (%s); an archived record is verbatim history and is never edited", a.Source, source)})
		}
	}
	return problems
}

// deletedRecords reports every durable record git history says was added and
// the tree no longer has.
//
// It is the guard contiguity cannot be: a record removed from the end of a
// series leaves the remaining numbers running 0001..N-1 with no gap, so the
// archive still looks whole from the inside. Only history remembers.
//
// It sees what this clone's history holds. A shallow clone reaches back only as
// far as it was fetched, so a deletion older than that window goes unreported;
// the workflow that runs these gates checks out with fetch-depth 0, which is
// where the guard has its full reach.
func deletedRecords(o Options, a archive) ([]Problem, error) {
	dirs := o.recordDirs()
	// No history means nothing was ever added, so nothing can have been
	// removed. Asking git anyway would fail on a repository with no commits.
	if len(dirs) == 0 || !o.Repo.Resolves("HEAD") {
		return nil, nil
	}
	added, err := o.Repo.PathsEverAdded(dirs...)
	if err != nil {
		return nil, err
	}
	var problems []Problem
	for _, rel := range added {
		if !specFileName.MatchString(path.Base(rel)) {
			continue
		}
		if existsExactly(o.RepoRoot, filepath.FromSlash(rel)) {
			continue
		}
		carrier, accounted := a.Deleted[rel]
		if !accounted {
			problems = append(problems, Problem{File: rel,
				Message: "was added and is no longer in the tree; spec_method.md keeps a superseded record in place marked Retired and never deletes it"})
			continue
		}
		if carrier == "" || !existsExactly(o.RepoRoot, filepath.FromSlash(carrier)) {
			problems = append(problems, Problem{File: rel,
				Message: fmt.Sprintf("%s accounts for this deletion by naming %q, which is not in the tree; the entry has to point at the record carrying the decision", a.Source, carrier)})
		}
	}
	return problems, nil
}

// recordDirs is the record directories as git names them: slash paths relative
// to the repository root.
//
// A directory configured outside the repository is dropped rather than asked
// about. git refuses a pathspec that leaves the tree, so passing one would turn
// an unusual configuration into an error from the gate rather than a question
// it simply cannot ask of this repository's history.
func (o Options) recordDirs() []string {
	var dirs []string
	for _, dir := range []string{o.SpecsDir, o.ADRDir} {
		rel := o.prefix(dir)
		if rel == "" || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
			continue
		}
		dirs = append(dirs, rel)
	}
	return dirs
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
