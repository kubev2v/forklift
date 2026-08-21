# logger -- structured logging setup

Shared across all four pipeline binaries (kc-prepare, kc-convert-linux, kc-convert-windows, kc-finalize) and the kc-v2v orchestrator. Lives in `common/` because every binary needs identical log initialization; duplicating it per stage would create drift.

Initializes the global `slog` logger for the entire application. Every binary in the pipeline calls `Init` once at startup to set the verbosity level, ensuring consistent structured log output across all stages.

The `Init` function maps a human-readable level string (`"debug"`, `"info"`, `"warn"`, `"error"`) to the corresponding `slog.Level`, creates a `TextHandler` that writes to stderr, and installs it as the default logger via `slog.SetDefault`. If the level string is unrecognized, it defaults to `info`.

## Key exports

| Symbol | Role |
|--------|------|
| `Init(level string)` | Configure the global slog logger with the given verbosity level |
