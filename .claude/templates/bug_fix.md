# Bug Fix Checklist — Higth

## 1. Reproduce First
- [ ] Bug is reproducible — document exact reproduction steps
- [ ] Expected behavior clearly stated
- [ ] Actual broken behavior clearly stated
- [ ] Reproduction in development environment only

## 2. Root Cause Analysis
- [ ] Identify the exact line(s) causing the issue
- [ ] Determine why the bug occurs (not just what happens)
- [ ] Check if the same bug exists in related code paths
- [ ] Assess security implications:
  - Does this expose data? (injection, auth bypass, information leak)
  - If yes → escalate to user immediately before proceeding

## 3. Fix
- [ ] Minimal, targeted fix — no opportunistic refactoring
- [ ] Fix resolves only the stated bug
- [ ] Fix does not introduce new behavior

## 4. Verify
- [ ] Bug no longer reproducible with fix applied
- [ ] `go build ./cmd/api` succeeds
- [ ] Related endpoints still function (regression check)
- [ ] Run smoke benchmark: `./tests/run-benchmarks.sh --tier smoke` (if relevant)

## 5. Document
- [ ] Root cause and fix summarized in commit message
- [ ] `state/DECISIONS_LOG.md` updated if root cause reveals architectural insight
- [ ] `state/CURRENT_STATUS.md` updated
- [ ] If the bug was in a pattern that appears elsewhere, note it for future review
