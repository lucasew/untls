## IGNORE: Refactor GetFreePort

**- Pattern:** Flattening logic or removing named return parameters in `GetFreePort`.
**- Justification:** The function is a direct copy from a Gist. Refactoring it obscures the origin and makes validation against the source harder.
**- Files Affected:** `listener.go`

## IGNORE: Enforce TLS Version

**- Pattern:** Setting `MinVersion` in `tls.Config`.
**- Justification:** As a generic proxy client, the tool must support whatever the upstream server supports. Enforcing TLS 1.2 breaks compatibility with legacy upstreams.
**- Files Affected:** `main.go`

## IGNORE: Custom Timeout Wrappers

**- Pattern:** Implementing `net.Conn` wrappers (like `idleTimeoutConn`) to handle timeouts.
**- Justification:** Custom connection wrappers add significant boilerplate complexity and risk introducing bugs. The project prefers simplicity over complex timeout handling unless critical.
**- Files Affected:** `main.go`

## IGNORE: Change Server Loop Exit Behavior

**- Pattern:** Changing `return` to `continue` (or other recovery logic) when `tls.Dial` fails in the main loop.
**- Justification:** The current behavior (exit on upstream failure) is intentional; changing it to keep the server running is consistently rejected.
**- Files Affected:** `main.go`

## IGNORE: Move Flag Definitions

**- Pattern:** Moving `flag` definitions (e.g. `flag.IntVar`) out of `init()`.
**- Justification:** Flag definitions belong in `init()`; only `flag.Parse()` should be in `main()`.
**- Files Affected:** `main.go`

## IGNORE: Agent-Specific Journals

**- Pattern:** Creating or updating agent-specific journal files (e.g., `.jules/janitor.md`, `.jules/sentinel.md`).
**- Justification:** Project conventions dictate a centralized approach rather than disparate agent-specific tracking files which clutter the repository. Such journal files are consistently rejected.
**- Files Affected:** `.jules/*.md` (excluding centralized files like `CONSISTENTLY_IGNORED.md`)

## IGNORE: Update Dependency Digests

**- Pattern:** Updating Docker image or GitHub Action dependencies by SHA digest.
**- Justification:** These PRs are consistently autoclosed, indicating the repository does not accept automated digest updates.
**- Files Affected:** `Dockerfile`, `.github/workflows/autorelease.yml`
