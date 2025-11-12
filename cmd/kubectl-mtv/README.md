# kubectl-mtv

A kubectl plugin for managing MTV (Migration Toolkit for Virtualization) resources.

## Building

### Development Build
```bash
make build
```

### Cross-Platform Builds

```bash
# Build for specific platforms
make build-linux     # Linux (AMD64 + ARM64)
make build-windows   # Windows (AMD64)
make build-darwin    # macOS (AMD64 + ARM64)

# Build all platforms
make build-all
```

### Update kubectl-mtv
```bash
make update-kubectl-mtv
```

## More Information

For complete documentation and usage examples, visit: https://github.com/yaacov/kubectl-mtv