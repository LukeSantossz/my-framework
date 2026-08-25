package usage

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestParsesTheOpenAIShapeIntoDisjointBuckets(t *testing.T) {
	// Cached input arrives inside prompt_tokens on this shape. Counting it in
	// both buckets would double the total and hide the very saving the gate's
	// message ordering exists to produce.
	raw := []byte(`{"usage":{"prompt_tokens":1000,"completion_tokens":300,
	 "prompt_tokens_details":{"cached_tokens":800},
	 "completion_tokens_details":{"reasoning_tokens":120}}}`)
	u := ParseOpenAI(raw)
	if !u.Known {
		t.Fatal("usage not recognised")
	}
	if u.InputTokens != 200 {
		t.Errorf("input = %d, want 200 (prompt minus cached)", u.InputTokens)
	}
	if u.CacheReadTokens != 800 {
		t.Errorf("cache-read = %d, want 800", u.CacheReadTokens)
	}
	if u.OutputTokens != 180 {
		t.Errorf("output = %d, want 180 (completion minus reasoning)", u.OutputTokens)
	}
	if u.ReasoningTokens != 120 {
		t.Errorf("reasoning = %d, want 120", u.ReasoningTokens)
	}
	if u.Total() != 1300 {
		t.Errorf("total = %d, want 1300; the buckets must partition the count", u.Total())
	}
	if u.Estimated {
		t.Error("a parsed figure must not be marked estimated")
	}
}

func TestParsesTheAnthropicShapeWithItsOwnFieldNames(t *testing.T) {
	raw := []byte(`{"usage":{"input_tokens":100,"output_tokens":50,
	 "cache_creation_input_tokens":10,"cache_read_input_tokens":900}}`)
	u := ParseAnthropic(raw)
	if !u.Known {
		t.Fatal("usage not recognised")
	}
	if u.CacheReadTokens != 900 || u.CacheWriteTokens != 10 {
		t.Errorf("cache buckets wrong: %+v", u)
	}
}

func TestParsesTheGoogleShapeWithItsOwnFieldNames(t *testing.T) {
	raw := []byte(`{"usageMetadata":{"promptTokenCount":500,"candidatesTokenCount":80,
	 "cachedContentTokenCount":400,"thoughtsTokenCount":25}}`)
	u := ParseGoogle(raw)
	if !u.Known {
		t.Fatal("usage not recognised")
	}
	if u.InputTokens != 100 || u.CacheReadTokens != 400 || u.ReasoningTokens != 25 {
		t.Errorf("buckets wrong: %+v", u)
	}
}

func TestAResponseWithNoUsageIsUnknownRatherThanZero(t *testing.T) {
	// Reporting zero as a measured value would be a fabricated number.
	for name, u := range map[string]Usage{
		"openai":    ParseOpenAI([]byte(`{"choices":[]}`)),
		"anthropic": ParseAnthropic([]byte(`{"content":[]}`)),
		"google":    ParseGoogle([]byte(`{"candidates":[]}`)),
		"garbage":   ParseOpenAI([]byte(`not json`)),
	} {
		if u.Known {
			t.Errorf("%s: reported known usage from a response carrying none: %+v", name, u)
		}
	}
}

func TestEstimateIsMarkedAsAnEstimateEverywhereItAppears(t *testing.T) {
	u := Estimate(4000, 400)
	if !u.Known || !u.Estimated {
		t.Fatalf("estimate = %+v, want known and estimated", u)
	}
	if !strings.Contains(u.String(), "ESTIMATED") {
		t.Errorf("rendering %q does not distinguish an estimate from a measurement", u.String())
	}
}

func TestAnEstimateContaminatesAnyTotalItJoins(t *testing.T) {
	// A total containing an estimate is an estimate. Losing the marking in the
	// aggregate corrupts every comparison built on it later.
	measured := ParseOpenAI([]byte(`{"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
	total := measured.Add(Estimate(400, 40))
	if !total.Estimated {
		t.Error("a total containing an estimate must report as estimated")
	}
}

func TestAddIgnoresAnUnknownFigure(t *testing.T) {
	measured := ParseOpenAI([]byte(`{"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
	total := measured.Add(Usage{})
	if total.Total() != measured.Total() {
		t.Errorf("an unknown figure changed the total: %+v", total)
	}
}

func TestUnknownUsageRendersAsUnknown(t *testing.T) {
	if got := (Usage{}).String(); !strings.Contains(got, "unknown") {
		t.Errorf("String() = %q, want it to say the usage is unknown", got)
	}
}

// --- money ------------------------------------------------------------------

func TestCostIsComputedOnlyFromAUserSuppliedTable(t *testing.T) {
	u := Usage{InputTokens: 1_000_000, OutputTokens: 1_000_000, Known: true}
	if _, ok := (Table{}).Cost("deepseek-v4", u); ok {
		t.Error("a cost was produced with no price table; a figure the tool cannot defend must not be printed")
	}
	table := Table{"deepseek-v4": {InputPerMillion: 2, OutputPerMillion: 8}}
	cost, ok := table.Cost("deepseek-v4", u)
	if !ok {
		t.Fatal("no cost from a table that names the model")
	}
	if cost != 10 {
		t.Errorf("cost = %v, want 10", cost)
	}
}

func TestCachedTokensFallBackToTheInputRateWhenNoneIsGiven(t *testing.T) {
	u := Usage{CacheReadTokens: 1_000_000, Known: true}
	table := Table{"m": {InputPerMillion: 3}}
	cost, _ := table.Cost("m", u)
	if cost != 3 {
		t.Errorf("cost = %v, want 3", cost)
	}
	table["m"] = Price{InputPerMillion: 3, CachedPerMillion: 0.3}
	cost, _ = table.Cost("m", u)
	if cost != 0.3 {
		t.Errorf("cost = %v, want 0.3 when a cached rate is given", cost)
	}
}

func TestNoCostFromAnUnknownFigure(t *testing.T) {
	if _, ok := (Table{"m": {InputPerMillion: 1}}).Cost("m", Usage{}); ok {
		t.Error("a cost was produced from usage nobody could determine")
	}
}

// --- the running total ------------------------------------------------------

func TestStoreAccumulatesAcrossRuns(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "usage.json")}
	if got := s.Read(); got.Runs != 0 {
		t.Errorf("a fresh store reported %d runs", got.Runs)
	}
	measured := Usage{InputTokens: 10, OutputTokens: 5, Known: true}
	if err := s.Add(measured); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(measured); err != nil {
		t.Fatal(err)
	}
	got := s.Read()
	if got.Runs != 2 || got.Usage.InputTokens != 20 {
		t.Errorf("totals = %+v, want 2 runs and 20 input tokens", got)
	}
}

func TestStoreDoesNotCountARunWhoseUsageIsUnknown(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "usage.json")}
	if err := s.Add(Usage{}); err != nil {
		t.Fatal(err)
	}
	if got := s.Read(); got.Runs != 0 {
		t.Errorf("an unknown figure was counted as a run: %+v", got)
	}
}

func TestStoreKeepsTheEstimatedMarkingThroughTheAggregate(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "usage.json")}
	if err := s.Add(Usage{InputTokens: 5, Known: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(Estimate(400, 40)); err != nil {
		t.Fatal(err)
	}
	if !s.Read().Usage.Estimated {
		t.Error("the cumulative total lost the estimated marking")
	}
}

func TestStoreDoesNotLoseARunToAConcurrentWriter(t *testing.T) {
	// A dropped run is invisible after the fact: the file that lost it still
	// parses, still looks like a plausible total, and nothing anywhere says a
	// number went missing. That is why this is asserted rather than trusted.
	s := Store{Path: filepath.Join(t.TempDir(), "usage.json")}
	const writers, each = 8, 25

	var start, done sync.WaitGroup
	start.Add(1)
	failures := make(chan error, writers)
	for i := 0; i < writers; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			for j := 0; j < each; j++ {
				if err := s.Add(Usage{InputTokens: 1, Known: true}); err != nil {
					failures <- err
					return
				}
			}
		}()
	}
	start.Done()
	done.Wait()
	close(failures)
	for err := range failures {
		t.Fatalf("Add: %v", err)
	}

	got := s.Read()
	if got.Runs != writers*each {
		t.Errorf("runs = %d, want %d; %d run(s) were overwritten by a concurrent writer",
			got.Runs, writers*each, writers*each-got.Runs)
	}
	if got.Usage.InputTokens != writers*each {
		t.Errorf("input tokens = %d, want %d; tokens were lost with the runs that carried them",
			got.Usage.InputTokens, writers*each)
	}
}

// usageChildEnv names the store a re-executed copy of this test binary should
// write to. Its presence is what turns the test below into the child half of
// the experiment rather than the parent half.
const usageChildEnv = "MF_USAGE_CONCURRENT_CHILD"

const (
	childProcesses = 4
	childAdds      = 20
)

func TestStoreDoesNotLoseARunToAConcurrentProcess(t *testing.T) {
	// The reported failure is two processes, not two goroutines: r2 and r3 run
	// as parallel CI jobs and share one usage.json. A lock that lives inside a
	// single process would satisfy the test above while leaving that defect
	// exactly where it was, so the real shape is reproduced here.
	if path := os.Getenv(usageChildEnv); path != "" {
		child := Store{Path: path}
		for i := 0; i < childAdds; i++ {
			if err := child.Add(Usage{InputTokens: 1, Known: true}); err != nil {
				t.Fatalf("child Add: %v", err)
			}
		}
		return
	}

	s := Store{Path: filepath.Join(t.TempDir(), "usage.json")}
	type child struct {
		cmd    *exec.Cmd
		output *bytes.Buffer
	}
	children := make([]child, 0, childProcesses)
	for i := 0; i < childProcesses; i++ {
		var out bytes.Buffer
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
		cmd.Env = append(os.Environ(), usageChildEnv+"="+s.Path)
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Start(); err != nil {
			t.Fatalf("starting a writer process: %v", err)
		}
		children = append(children, child{cmd: cmd, output: &out})
	}
	for _, c := range children {
		if err := c.cmd.Wait(); err != nil {
			t.Fatalf("a writer process failed: %v\n%s", err, c.output.String())
		}
	}

	want := childProcesses * childAdds
	if got := s.Read(); got.Runs != want {
		t.Errorf("runs = %d, want %d; %d run(s) were overwritten by another process",
			got.Runs, want, want-got.Runs)
	}
}

func TestStoreResets(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "usage.json")}
	if err := s.Add(Usage{InputTokens: 5, Known: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if s.Read().Runs != 0 {
		t.Error("reset left a total behind")
	}
	if err := s.Reset(); err != nil {
		t.Errorf("resetting an already-empty store failed: %v", err)
	}
}
