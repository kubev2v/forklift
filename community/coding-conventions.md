# Coding Conventions

Practical conventions for contributing to **Migration Toolkit for Virtualization (MTV)** / the [Forklift](https://github.com/kubev2v/forklift) codebase. These rules come from maintainer review feedback and merged PRs. They supplement—not replace—[code quality](code-quality.md), [code review](code-review.md), and [AI contribution policy](ai-contribution-policy.md).

**Audience:** human contributors and AI assistants. When generating or reviewing code, apply these conventions by default unless a maintainer approves an exception.

PR scope, design approval, and review process are in [code review](code-review.md)—not repeated here.

---

## Logging & observability

### Log `name` and `namespace` as separate fields

Pass resource identity as structured key/value pairs, not a single combined string:

```go
// Good
log.Info("reconciling", "name", resource.GetName(), "namespace", resource.GetNamespace())

// Bad — debug tooling cannot parse a combined label reliably
log.Info("reconciling", "resource", resource.GetNamespace()+"/"+resource.GetName())
```

Debug and observability tooling expects `name` and `namespace` as individual fields. ([PR #5555](https://github.com/kubev2v/forklift/pull/5555))

---

## Error & context handling

### Use the reconciler/request `ctx`, not `context.Background()`

When a function already receives `ctx`, do not substitute `context.Background()` for downstream calls. SonarCloud flags this, and reviewers treat it as a real bug: work started under a fresh background context can continue after the parent context (or lock) has been cancelled. ([PR #6689](https://github.com/kubev2v/forklift/pull/6689))

### Check HTTP status before decoding the body

For outbound HTTP, inspect `resp.StatusCode` (or `resp.Status`) before `json.Decoder` or similar. Otherwise `401`/`500` responses produce misleading JSON parse errors instead of the actual failure. ([PR #6839](https://github.com/kubev2v/forklift/pull/6839))

### External HTTP clients must have a timeout

Clients used from controllers or other reconcile paths must not block indefinitely. A client with no timeout can stall reconcile loops on network hangs. Set `Timeout` on `http.Client` (or use a context deadline on the request). ([PR #6839](https://github.com/kubev2v/forklift/pull/6839))

---

## Type safety

### Use the comma-ok idiom for interface assertions

```go
// Good
ok, v := x.(bool)
if !ok { ... }

// Bad — panics on unexpected dynamic types
v := x.(bool)
```

([PR #6688](https://github.com/kubev2v/forklift/pull/6688))

### Use `%v` for non-string values in `fmt.Errorf`

Slices, structs, and other non-strings should use `%v` (or a type-specific verb), not `%s`. ([PR #6689](https://github.com/kubev2v/forklift/pull/6689))

---

## Constructor & input validation

### Reject invalid constructor inputs at construction time

Do not accept inputs that will fail later as a panic or hang—for example `nil` lockers, zero concurrency limits, or empty required prefixes. Validate in the constructor and return an error. ([PR #6689](https://github.com/kubev2v/forklift/pull/6689))

### Validate that referenced Kubernetes resources exist

Non-empty field values are not enough. For references such as `StorageClass`, confirm the object exists in the API server. A typo passes a nil/empty check and can leave workloads (e.g. PVCs) stuck `Pending` with no clear validation error. ([PR #6839](https://github.com/kubev2v/forklift/pull/6839))

---

## API & resource lookup

### Prefer direct ID-based lookup over scanning a capped list

Do not find a resource by listing the first *N* items and scanning in memory. Once the collection exceeds *N*, valid IDs are reported as “not found.” Use a direct get-by-ID API when the platform supports it. ([PR #6845](https://github.com/kubev2v/forklift/pull/6845))

### Do not expose API enum values for unimplemented features

Unsupported enum values in CRDs or APIs are accepted by admission but fail silently at runtime. Omit or gate values until the feature is implemented. ([PR #6839](https://github.com/kubev2v/forklift/pull/6839))

---

## Settings & configuration

### Environment-driven settings belong in `pkg/settings/`

Centralize environment variable parsing and defaults in [`pkg/settings/`](../pkg/settings/). Do not add package-local `init()` blocks that read env vars in individual packages. ([PR #6662](https://github.com/kubev2v/forklift/pull/6662))

---

## Testing

### Restore global settings after tests; do not hardcode “defaults”

When tests mutate package-level settings, save the previous value and restore it in `defer`:

```go
prev := settings.SomeFlag
defer func() { settings.SomeFlag = prev }()
```

Resetting to a fixed literal can break test order and hide coupling between tests. ([PR #6689](https://github.com/kubev2v/forklift/pull/6689))

---

## Go style

### Prefer explicit `return err` over named result parameters

Use named returns only when they materially simplify the function (e.g. deferred cleanup that sets both result and error). ([PR #5653](https://github.com/kubev2v/forklift/pull/5653))

### Use `ca.crt` for CA certificates in Secrets

The field name `ca.crt` is standard; `cacert` is deprecated in this project. ([PR #5549](https://github.com/kubev2v/forklift/pull/5549))

### Replace magic numbers with named constants

Numeric literals with domain meaning (limits, timeouts, thresholds) should be named constants at package or file scope. ([PR #6136](https://github.com/kubev2v/forklift/pull/6136))

### Keep declarations at the top of the function

Declare variables at the start of the function (after imports and before logic), not interleaved mid-flow. This matches common Go style in the tree and makes control flow easier to scan.

---

## Reconciler patterns

### No sleep, retry, or poll loops in `Reconcile`

Try the operation once, update status/state, and return—do not use `time.Sleep`, busy retry, spin, or `for` loops that wait until an external condition becomes true. The reconcile loop (and `Result.RequeueAfter`) is the poll mechanism. ([PR #5902](https://github.com/kubev2v/forklift/pull/5902))

Do not use `for` loops in place of direct API lookups; see [API & resource lookup](#api--resource-lookup).

### Avoid finalizers unless explicitly required

Prefer status conditions, owner references, and natural API deletion semantics. Finalizers add ordering complexity, failure modes on stuck cleanup, and upgrade risk. Add a finalizer only when the design requires guaranteed pre-delete work and that behavior is agreed with maintainers.

### Avoid mutexes in controllers

Controller-runtime already serializes reconciliation per object key. Do not add `sync.Mutex` / `RWMutex` around reconcile paths to “make it safe”—it usually signals the wrong decomposition. If shared mutable state seems necessary, prefer passing dependencies through the reconciler struct, using the API server as source of truth, or narrowing the design before introducing locks.

---

## Security

### Do not build SSRF-style proxies from caller-supplied URLs

Endpoints must not fetch arbitrary URLs provided by the client. Resolve external service URLs server-side from trusted configuration (CRs, operator settings, cluster config). ([PR #5864](https://github.com/kubev2v/forklift/pull/5864))

---

## Quick checklist (PR / AI self-review)

Use this before opening or approving a PR:

| Area | Check |
|------|--------|
| Logging | `name` and `namespace` logged as separate structured fields |
| Context | No `context.Background()` where `ctx` is already in scope |
| HTTP | Status checked before body decode; client has timeout |
| Types | Interface assertions use comma-ok; `%v` for non-strings in errors |
| Validation | Constructors reject bad inputs; K8s references verified to exist |
| APIs | No capped list scans for lookup; no unimplemented enum values exposed |
| Settings | Env config only in `pkg/settings/` |
| Tests | Global settings saved and restored in `defer` |
| Go style | Explicit error returns; `ca.crt`; magic numbers → constants; declarations at top |
| Reconcile | No sleep/retry/poll loops; avoid finalizers and mutexes unless justified |
| Security | No user-controlled outbound URLs |
| Process | PR scope and comments per [code review](code-review.md) |

---

## References

- [Code quality](code-quality.md) — local CI, lint, generate, manifests
- [Code review](code-review.md) — design-first process and reviewer expectations
- [AI contribution policy](ai-contribution-policy.md) — disclosure and ownership of AI-assisted work
- [Kubernetes coding conventions](https://github.com/kubernetes/community/blob/master/contributors/guide/coding-conventions.md)
