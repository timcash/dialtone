# Logs Plugin

This plugin is a **shared library** and **CLI** for standardized logging across Dialtone. It unifies browser and backend logs (NATS topics, xterm streaming, HTTP fallback) and is tested with its own CLI and UI.

- **Current version**: `src_v1`
- **README**: [src_v1/README.md](src_v1/README.md)

## Quick start

```bash
./dialtone.sh logs install src_v1
./dialtone.sh logs test src_v1
./dialtone.sh logs dev src_v1
```

Every plugin must include a `README.md` at its plugin root; the detailed design and CLI/test flow live in the versioned README above.
