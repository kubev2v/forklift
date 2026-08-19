# config -- Environment variable constants and Config struct

Centralizes the Forklift V2V environment variable names, default paths, and the `Config` struct used throughout the kc-v2v pipeline. Every `V2V_*` environment variable consumed by the converter is declared here as a named constant.

The package provides no logic beyond type definitions and constants. The `Config` struct mirrors the Forklift `AppConfig` fields relevant to kc-v2v: source credentials, disk paths, firmware hints, feature flags (overlay, LUKS/Clevis, VMware driver removal, static IPs), and working-directory locations. Default values are defined as constants (e.g. `DefaultWorkdir`, `DefaultCopyConcurrency`) and referenced by the `env.Load` function that populates `Config` at startup.

## Key exports

| Symbol | Role |
|--------|------|
| `Config` | Struct holding all kc-v2v runtime configuration fields |
| `EnvLibvirtURL`, `EnvSource`, `EnvVmName`, ... | String constants for each `V2V_*` environment variable name |
| `DefaultWorkdir` | Default working directory (`/var/tmp/v2v`) |
| `DefaultCopyConcurrency` | Default parallel disk copy limit (4) |
| `DefaultCaBundle` | Symlink target for `LinkCertificates` (`/opt/ca-bundle.crt`) |
| `DefaultCaCert` | Mounted provider PEM path for TLS and `LinkCertificates` source (`/etc/secret/cacert`) |
| `DefaultInspectionOutputFile` | Default path for the inspection XML output |
| `DefaultLuksDir` | Default directory for LUKS key files |
| `DefaultDynamicScriptsDir` | Default directory for dynamic customization scripts |
| `DefaultMountRoot` | Default guest mount root path |
| `BlockGlob`, `FSGlob` | Glob patterns for discovering block devices and filesystem disk images |
