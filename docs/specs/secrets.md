# Secrets and 1Password integration

This repo follows one non-negotiable rule:

**Secret values never land in git.**

The repo may store *references* to secrets (1Password item/field identifiers), but never secret material.

---

## Threat model (what we’re protecting against)

- Accidental commits of tokens/keys/passwords.
- “Auto capture” (`chezmoi re-add`) pulling secrets from destination files back into repo source state.
- Machine compromise is out of scope (if an attacker owns your box, they can read your decrypted secrets), but we still aim to minimize secret sprawl.

---

## Recommended secret patterns

### 1) Split config into public + private include

Preferred whenever the tool supports includes.

Example: Git

- Managed in repo: `~/.gitconfig`
- Generated locally (gitignored): `~/.gitconfig.private`

`~/.gitconfig` contains:

```ini
[include]
    path = ~/.gitconfig.private
```

`dot apply` generates `.gitconfig.private` from 1Password and writes it to the destination.  
`dot capture` never tracks it because it is not managed by Chezmoi.

This approach keeps the main config editable and syncable across machines.

### 2) Generate full files into `state/private/` and copy to destination

For tools that do not support include files:

- Keep the template (non-secret) in repo, or keep a generator definition.
- At apply time:
  - fetch secrets from 1Password
  - render a full file under `state/private/<os>/<host>/...`
  - copy it into place

`state/private/` is gitignored.

---

## 1Password prerequisites

### Desktop app
Install and sign in to 1Password Desktop on:
- macOS
- Windows

Enable:
- SSH agent (so GitHub auth works without local private keys)
- CLI integration

### CLI (`op`)
Install 1Password CLI and connect it to the desktop app.

A “manual checkpoint” is expected during bootstrap:
- The user must authenticate once (QR / system auth prompt).
- After that, `dot apply` can fetch secrets as needed.

If the stable CLI is slow in your environment, the system may support an optional “beta CLI” install path.
This is intentionally opt-in because it increases update churn.

---

## How secret references are stored

In v1, references should live in one of:
- `dot.toml` (simple cases, small number of references)
- `state/secrets/refs.toml` (recommended as it grows)
- Per-module config files

A reference is always a tuple like:
- account (optional)
- item identifier (UUID or title)
- field name (or JSON pointer if needed)

Example (conceptual):

```toml
[secrets.git]
signing_key = "op://Personal/Git Signing Key/private_key"
```

The repo stores only these references.

---

## Caching and performance

1Password CLI calls can be slow.

Design requirement:
- `dot apply` should cache secret fetches **within a single run**.
- `dot sync` should avoid calling `op` unless an apply step truly needs it.
- Scheduled runs should be able to skip secret-heavy operations if the vault is locked.

---

## Guardrails

### Discover
`dot discover` must default to:
- `chezmoi add --secrets=error`
- Risky paths not preselected

### Capture
The managed set should not include secret-bearing files.
If a secret ends up in a managed file anyway, that’s a bug—treat it like a fire drill:
- remove secret from file
- rotate token/key
- consider adding a deny rule for that path

### Optional (recommended later): secret scanning
Add a pre-commit hook or CI check (gitleaks, etc.) as a last line of defense.
