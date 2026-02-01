# Documentation

Welcome to the dotstate documentation.

---

## Quick Navigation

### Getting Started

- **[Architecture](architecture.md)** - How dotstate works
- **[Roadmap](roadmap.md)** - Development phases and status

### Reference

- **[CLI Reference](reference/cli.md)** - Commands, flags, exit codes
- **[Configuration](reference/configuration.md)** - `dot.toml` schema, environment variables

### Specifications

Detailed specifications for each subsystem:

- **[Discover](specs/discover.md)** - Configuration discovery (`dot discover`)
- **[Sync](specs/sync.md)** - Sync transaction (`dot sync`)
- **[Scheduling](specs/scheduling.md)** - Automated sync cadence
- **[Secrets](specs/secrets.md)** - 1Password integration and secret handling
- **[Advanced Tracking](specs/advanced-tracking.md)** - Browser profiles, sub-repos, system settings

### Bootstrap Guides

Platform-specific setup instructions:

- **[macOS](guides/bootstrap-macos.md)** - Apple Silicon setup
- **[Windows](guides/bootstrap-windows.md)** - Windows 11 Pro setup
- **[Linux](guides/bootstrap-linux.md)** - CachyOS/Arch setup
- **[WSL](guides/wsl-nixos.md)** - NixOS-WSL integration

---

## Document Hierarchy

```
docs/
├── README.md                 # This file
├── architecture.md           # System architecture
├── roadmap.md               # Development phases
├── reference/
│   ├── cli.md               # CLI reference (authoritative)
│   └── configuration.md     # Configuration reference (authoritative)
├── specs/
│   ├── discover.md          # dot discover specification
│   ├── sync.md              # dot sync specification
│   ├── scheduling.md        # Scheduling specification
│   ├── secrets.md           # Secrets specification
│   └── advanced-tracking.md # Complex tracking scenarios
└── guides/
    ├── bootstrap-macos.md
    ├── bootstrap-windows.md
    ├── bootstrap-linux.md
    └── wsl-nixos.md
```

---

## Contributing to Documentation

### Single Source of Truth

Each piece of information should exist in exactly one place:

| Topic | Authoritative Location |
|-------|----------------------|
| CLI commands, flags, exit codes | `reference/cli.md` |
| Configuration options | `reference/configuration.md` |
| Development phases | `roadmap.md` |
| Architecture overview | `architecture.md` |
| Feature specifications | `specs/*.md` |

### When to Update

- **Adding a command/flag**: Update `reference/cli.md`
- **Changing configuration**: Update `reference/configuration.md`
- **Completing a phase**: Update `roadmap.md`
- **Architectural changes**: Update `architecture.md`

The root `README.md` should remain concise and link here for details.
