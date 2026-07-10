#!/usr/bin/env bash
# Reproducible model benchmark for SPEC 0009: gpt-5.5 vs gpt-5.6-terra vs
# gpt-5.6-sol as the R2 codex-review reviewer. Three seeded-defect diffs
# (correctness / invented-symbol / security) plus two real historical diffs.
# Runs `codex review --commit <SHA>` at model_reasoning_effort=high for each
# (model, diff) cell, capturing the verdict, latency, and exit code. Creates a
# throwaway branch for the seeded commits and deletes it on completion, so the
# working branch is left untouched. Requires an authenticated Codex CLI.
#
# Usage:   bash docs/specs/0009-assets/run_benchmark.sh
# Output:  $BENCH_OUT (default: a temp dir printed at the end); the summary is
#          written to $BENCH_OUT/manifest.csv in the same shape as the tracked
#          manifest.csv beside this script.
set -u

REPO="$(git rev-parse --show-toplevel)" || { echo "FATAL: not in a git repo"; exit 1; }
OUT="${BENCH_OUT:-${TMPDIR:-/tmp}/r2-model-bench}"
BRANCH="bench/model-eval-0009"
MODELS="${BENCH_MODELS:-gpt-5.5 gpt-5.6-terra gpt-5.6-sol}"
REAL_DIFFS="${BENCH_REAL_DIFFS:-c3a891b 5ff245c}"
mkdir -p "$OUT/seeds"
cd "$REPO" || exit 1

MANIFEST="$OUT/manifest.csv"
echo "label,model,sha,category,ground_truth,seconds,exit" > "$MANIFEST"

# --- Seed sources (each: exactly one planted defect) --------------------------
cat > "$OUT/seeds/seed1-correctness.sh" <<'EOF'
#!/usr/bin/env bash
# Guard: skip the review when we are on the base branch (nothing to review
# against itself); otherwise run the review for the current feature branch.
set -u
base="${REVIEW_BASE:-main}"
branch="$(git rev-parse --abbrev-ref HEAD)"

if [ "$branch" != "$base" ]; then
  echo "On base branch '$base'; nothing to review. Skipping."
  exit 0
fi

echo "Running review for $branch vs $base"
codex review --base "$base"
EOF

cat > "$OUT/seeds/seed2-invented.sh" <<'EOF'
#!/usr/bin/env bash
# Emit the review as JSON, tagged with the current branch name.
set -u
branch="$(git rev-parse --branch-name)"
codex review --base main --format=json > "review-${branch}.json"
echo "wrote review-${branch}.json"
EOF

cat > "$OUT/seeds/seed3-security.sh" <<'EOF'
#!/usr/bin/env bash
# Dispatch a named review hook supplied by the caller.
set -u
hook="${1:-}"
eval "review_hook_${hook}"
EOF

# --- Build the scratch branch with one commit per seed ------------------------
orig="$(git rev-parse --abbrev-ref HEAD)"
git branch -D "$BRANCH" >/dev/null 2>&1 || true
git checkout -q -b "$BRANCH" || { echo "FATAL: cannot create $BRANCH"; exit 1; }
mkdir -p scripts/bench

declare -A SEED_SHA
for n in 1 2 3; do
  case $n in
    1) f=seed1-correctness.sh; cat=correctness ;;
    2) f=seed2-invented.sh;    cat=invented-symbol ;;
    3) f=seed3-security.sh;    cat=security ;;
  esac
  cp "$OUT/seeds/$f" "scripts/bench/$f"
  git add "scripts/bench/$f"
  git commit -q -m "bench: seed $cat defect ($f)"
  SEED_SHA[$n]="$(git rev-parse HEAD)"
done

# --- Diffs under test: 3 seeded + 2 real --------------------------------------
# fields: label|sha|category|ground_truth
DIFFS=(
  "seed1_correctness|${SEED_SHA[1]}|correctness|DEFECT: guard condition inverted (skips feature branches, runs only on base)"
  "seed2_invented|${SEED_SHA[2]}|invented-symbol|DEFECT: codex review --format=json and git rev-parse --branch-name do not exist"
  "seed3_security|${SEED_SHA[3]}|security|DEFECT: eval of unsanitized \$1 -> command injection"
)
i=0
for real in $REAL_DIFFS; do
  i=$((i + 1))
  DIFFS+=("real_${real}|${real}|real-clean|CLEAN: pre-merged hardening commit, no real defect")
done

# --- Run every (model, diff) cell --------------------------------------------
for model in $MODELS; do
  for row in "${DIFFS[@]}"; do
    IFS='|' read -r label sha category truth <<< "$row"
    tag="${label}__${model}"
    t0=$(date +%s)
    codex review --commit "$sha" \
      -c "model=$model" -c "model_reasoning_effort=high" \
      </dev/null > "$OUT/$tag.txt" 2>&1
    ec=$?
    t1=$(date +%s)
    echo "$label,$model,$sha,$category,\"$truth\",$((t1 - t0)),$ec" >> "$MANIFEST"
    echo "done: $tag ($((t1 - t0))s, exit $ec)"
  done
done

# --- Cleanup: back to the original branch, drop the scratch branch ------------
git checkout -q "$orig"
git branch -D "$BRANCH" >/dev/null 2>&1 || true
echo "ALL DONE. Manifest: $MANIFEST"
