# Bootstrap macOS (Apple Silicon)

This doc brings a fresh macOS machine to a usable `dotstate` baseline.

Manual checkpoints you should expect:
- Xcode Command Line Tools install prompt
- 1Password sign-in/unlock (Touch ID / QR)

---

## Target end state

After bootstrap:

- `dot` is installed and on PATH (or available in repo `bin/`)
- `git`, `chezmoi`, and `op` are installed
- 1Password desktop + CLI integration is working
- `dot apply` works
- `dot discover` can harvest existing configs

---

## Bootstrap (MVP, today)

### 0) Install Xcode Command Line Tools (manual checkpoint)
Run:

```bash
xcode-select --install
```

Follow the prompt. This is required for many developer tools.

### 1) Install Homebrew (recommended)
Install Homebrew using the official instructions.

Then install dependencies:

```bash
brew install go git chezmoi
brew install --cask 1password
brew install --cask 1password-cli
```

If you prefer not to use Homebrew:
- install Go, Git, Chezmoi, 1Password, and 1Password CLI via official installers
- ensure `go`, `git`, `chezmoi`, and `op` are on PATH

### 2) Sign in to 1Password + enable SSH agent + CLI integration
- Sign in to 1Password desktop
- Enable SSH agent
- Enable CLI integration
- Verify:

```bash
op whoami
```

Authenticate if prompted.

### 3) Clone repo

```bash
git clone https://github.com/dnery/dotstate ~/Projects/dotstate
cd ~/Projects/dotstate
```

### 4) Build `dot`

```bash
make build
```

Or build just for macOS:

```bash
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ./bin/dot ./cmd/dot
```

### 5) Validate

```bash
./bin/dot doctor
./bin/dot apply
```

Then:

```bash
./bin/dot discover
```

---

## Bootstrap (target flow, later)

Eventually macOS bootstrap should be:

```bash
curl -fsSL <bootstrap-url> | sh
dot bootstrap --repo https://github.com/dnery/dotstate
```

Where the bootstrap script:
- downloads the correct `dot` binary from GitHub Releases
- installs prerequisites (brew)
- blocks on 1Password sign-in checkpoint
