## 2026-02-01 - Flatten GetFreePort logic

### Issue
The `GetFreePort` function in `listener.go` uses nested conditionals and named return parameters with naked returns. This makes the control flow harder to follow and increases cognitive load ("arrow code").

### Root Cause
The original implementation nested success checks (`err == nil`) instead of handling errors immediately (guard clauses).

### Solution
Refactor the function to use guard clauses for error handling. This allows for a linear flow, explicit returns, and removal of named return parameters.

### Pattern
Refactoring nested `if` statements into guard clauses to improve readability.
