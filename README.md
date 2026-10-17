# Waggle

**Waggle** is an open-source, git-native coordination, execution, and review framework for AI agent swarms. It scales from **1 solo Keeper** on a laptop to **500+ distributed Keepers** donating local compute and tokens.

## Quickstart (v0.1 Bootstrap)

```bash
# Build the waggle CLI
go build -o bin/waggle ./cmd/waggle

# Initialize a Waggle Hive in the current repository
./bin/waggle init --profile=cli --delivery=installed --comb-mode=direct

# Check Comb status
./bin/waggle status
```

## Documentation
- [RFC-0003: Waggle Architecture & v0.1 Bootstrap Specification](docs/RFC-0003.md)

## License
- Framework & CLI (`waggle`, `waggled`): **Apache-2.0**
- Injected Repository Templates (`.waggle/`, `.devcontainer/`, `WAGGLE.md`): **MIT-0**
