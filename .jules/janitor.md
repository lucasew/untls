## 2026-01-27 - Refactor flag parsing from init() to main()

### Issue
Flag parsing was done in `init()`, which executes automatically on package import. This makes it difficult to test `main` package logic without triggering flag parsing and potential side effects (like program exit on error).

### Root Cause
Using `init()` for side-effect heavy initialization like flag parsing and network checks (`GetFreePort`).

### Solution
Moved flag parsing logic to a dedicated `parseFlags()` function and called it explicitly at the beginning of `main()`.

### Pattern
Avoid side effects in `init()`. Prefer explicit initialization.
