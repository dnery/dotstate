# WSL (NixOS‑WSL) target spec

WSL is optional, but treated as a **first‑class target** because Docker and Linux tooling are unavoidable.

The model:

- Windows host is “primary”.
- WSL (NixOS‑WSL) is a separate environment with its own state (packages, home-manager config, etc.).
- The dotstate repo owns both, so you don’t maintain a second repo just for WSL.

This doc describes the target behavior and the manual checkpoints required.

---

## High-level goals

1. Install a NixOS‑WSL distribution in a repeatable way.
2. Switch the distro to flakes safely (minimal brick risk).
3. Apply a repo-provided flake that configures:
   - WSL integration settings (docker desktop, start menu launchers, etc.)
   - system packages
   - home-manager setup for the user
4. Integrate 1Password usage sensibly:
   - prefer Windows-host 1Password SSH agent
   - avoid scattering private keys into WSL

---

## Proposed repo layout for WSL

Add a WSL target directory in this repo:

- `targets/wsl/flake.nix`
- `targets/wsl/flake.lock`
- `targets/wsl/configuration.nix` (or a module referenced by the flake)
- `targets/wsl/home.nix`
- `state/private/wsl/` (gitignored) for locally generated secrets, if needed

The WSL target should be referenced by `dot.toml`:

```toml
[wsl]
enable = true
distro_name = "nixos"
flake_ref = ".#wsl" # or "./targets/wsl#wsl"
```

---

## Install flow (what `dot wsl install` should do)

### 0) Preconditions
- Windows 11 Pro with WSL feature available
- Docker Desktop installed (optional but recommended)

### 1) Install/enable WSL
If WSL is not installed/enabled, run:

- `wsl --install --no-distribution`

(If the system requires a reboot, this is a manual checkpoint.)

### 2) Install NixOS‑WSL distribution
Preferred:
- Download the latest `nixos.wsl` release artifact and install it (double click)  
Fallback:
- `wsl --install --from-file nixos.wsl`

Then set it as default:
- `wsl -s nixos`

### 3) Prepare `/etc/nixos/configuration.nix` for flakes + WSL settings
Edit `/etc/nixos/configuration.nix` and ensure:
- `wsl.enable = true;`
- docker desktop integration enabled (if desired)
- default user configured (`wsl.defaultUser`)
- enable flakes in `nix.settings.experimental-features`

This step is safety-critical; keep it explicit and documented.

### 4) “Boot then restart” rebuild (safe switch-over)
Run:

- `sudo nixos-rebuild boot`
- terminate the distro: `wsl -t nixos`
- start root once and exit: `wsl -d nixos --user root exit`
- terminate again: `wsl -t nixos`

This ensures the generation is applied cleanly before you start doing flake switches.

### 5) Enable Docker Desktop distro integration (manual)
In Docker Desktop settings:
- enable integration for the `nixos` distro.

### 6) Switch to repo-owned flake
Inside WSL:
- clone the dotstate repo (or use a Windows-mounted checkout)
- run: `sudo nixos-rebuild --flake <flakeRef> switch`

After switch:
- terminate WSL and re-enter (same pattern as above) if needed.

### 7) First home-manager generation
Inside WSL:
- run home-manager switch with the repo flake target.

---

## 1Password and SSH inside WSL

Preferred model:
- Use the Windows host’s 1Password SSH agent.
- In WSL, use Windows `ssh.exe` for Git operations.

This avoids copying private keys into WSL.

If you need a signing key or token inside WSL:
- fetch it via 1Password CLI on Windows and write it into `state/private/wsl/...` (gitignored),
- then copy it into WSL as needed.

---

## Manual checkpoints (explicit)

- Windows reboot after enabling WSL (if required)
- Docker Desktop integration toggle
- 1Password sign-in / unlock (once)
- Any operation that modifies `/etc/nixos/configuration.nix` or rebuilds NixOS (requires sudo)

---

## Non-goals (for now)

- Full “auto-heal” of broken WSL distros
- Managing WSL kernel / advanced networking beyond Docker Desktop needs
