# Bootstrap Linux (CachyOS / Arch-family)

This doc brings a fresh Arch-family Linux machine (CachyOS) to a usable `dotstate` baseline.

Manual checkpoints you should expect:
- sudo password prompts
- optional 1Password sign-in (if using the desktop app)

---

## Target end state

After bootstrap:

- `dot` is installed and runnable
- `git`, `chezmoi`, and `op` are installed
- `dot apply` works
- `dot discover` can harvest configs
- package manifests can be captured and applied later (pacman explicit + foreign lists)

---

## Bootstrap (MVP, today)

### 0) Install prerequisites
Using pacman:

```bash
sudo pacman -Syu --needed git go chezmoi
```

Install 1Password CLI (`op`):
- Either via your preferred package source (AUR/package) or official installer.
- Ensure `op` is on PATH.

### 1) 1Password login (manual checkpoint)
Depending on your 1Password setup:
- If using 1Password desktop app: enable CLI integration and sign in.
- If using CLI-only: authenticate once.

Verify:

```bash
op whoami
```

### 2) Clone repo

```bash
git clone https://github.com/dnery/dotstate ~/Projects/dotstate
cd ~/Projects/dotstate
```

### 3) Build `dot`

```bash
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ./bin/dot ./cmd/dot
```

Or use `make build` if you want cross-build artifacts:

```bash
make build
```

### 4) Validate

```bash
./bin/dot doctor
./bin/dot apply
./bin/dot discover
```

---

## Bootstrap (target flow, later)

Eventually Linux bootstrap should be:

```bash
curl -fsSL <bootstrap-url> | sh
dot bootstrap --repo https://github.com/dnery/dotstate
```

Where the bootstrap script:
- downloads the appropriate `dot` binary
- installs prerequisites via pacman
- blocks on 1Password checkpoint
