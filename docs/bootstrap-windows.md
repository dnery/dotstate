# Bootstrap Windows (11 Pro)

This doc is the step-by-step for bringing a fresh Windows machine to a usable `dotstate` baseline.

The goal is eventually “1–2 commands”, but Windows still requires a few unavoidable manual checkpoints:
- Windows activation (legitimate)
- 1Password sign-in/unlock (QR / Windows Hello)
- occasional reboot when enabling WSL or applying certain system features

---

## Target end state

After completing this bootstrap:

- `dot` is installed and on PATH.
- `git`, `chezmoi`, and `op` are installed and usable.
- 1Password desktop is installed, signed in, and SSH agent works.
- `dot apply` works from the repo.
- `dot discover` can be run to harvest existing configs.
- (Optional) WSL NixOS target can be installed and managed.

---

## Bootstrap (MVP, today)

### 0) Manual checkpoint: Windows activation status
This repo will *not* automate activation circumvention.

Do this now:
- Settings → System → Activation
- Ensure Windows is activated (or activate it legitimately)

Then continue.

### 1) Install prerequisites (recommended via winget)
Open an elevated PowerShell (Admin) only if winget is missing; otherwise normal PowerShell is fine.

Install:
- Git
- 1Password Desktop
- 1Password CLI
- Chezmoi

You can do this via winget packages (names may vary):
- `winget install Git.Git`
- `winget install 1Password.1Password`
- `winget install 1Password.CLI`
- `winget install twpayne.chezmoi` (or equivalent)

If you prefer manual installs, that’s fine; the only requirement is that `git`, `chezmoi`, and `op` are on PATH.

### 2) Sign in to 1Password + enable SSH agent
- Sign in to the desktop app
- Enable SSH agent
- Enable CLI integration
- Verify:

```powershell
op whoami
```

(If prompted, authenticate and re-run.)

### 3) Clone repo
Pick a consistent location. Example:

```powershell
git clone https://github.com/dnery/dotstate "$HOME\Projects\dotstate"
cd "$HOME\Projects\dotstate"
```

### 4) Build `dot`
Use your `make.ps1` or `go build` directly.

Example:

```powershell
go build -o .\bin\dot.exe .\cmd\dot
```

(Optional) Put `dot.exe` on PATH (or use the repo-local binary).

### 5) Validate
Run:

```powershell
.\bin\dot.exe doctor
```

Then:

```powershell
.\bin\dot.exe apply
```

At this point you can run:

```powershell
.\bin\dot.exe discover
```

---

## Bootstrap (target flow, later)

Eventually, Windows bootstrap should be:

```powershell
irm <bootstrap-url> | iex
dot bootstrap --repo https://github.com/dnery/dotstate
```

Where the bootstrap script:
- downloads a prebuilt `dot.exe` from GitHub Releases
- installs prerequisites via winget
- runs `dot doctor`
- blocks on manual checkpoints (activation, 1Password sign-in)

---

## Optional: WSL (NixOS-WSL)

If you want WSL, follow `docs/wsl-nixos.md` (or run `dot wsl install` once implemented).

---

## Optional: PowerShell 7 + Windows Terminal as default

This is a Windows-only module. The intent:
- install PowerShell 7
- install Windows Terminal
- set Terminal’s default profile to PowerShell 7
- (best effort) set Terminal as the default terminal application

This is documented separately once implemented.
