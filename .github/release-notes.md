Instaladores do **envsetup**, o app que monta e restaura o PC depois de formatar.

**Windows** (PowerShell):

```powershell
irm https://github.com/EriikGabriel/dev-environment-files/releases/latest/download/install.ps1 | iex
```

ou baixe o `envsetup_*_x64-setup.exe` abaixo.

**Ubuntu / Debian e Arch** (terminal):

```bash
curl -fsSL https://github.com/EriikGabriel/dev-environment-files/releases/latest/download/install.sh | bash
```

ou baixe o `.deb` (Ubuntu) ou o `.AppImage` (Arch e outros) abaixo.

O que o app faz numa instalação limpa, e como usar com as configurações de outra pessoa:
[docs/formatacao.md](https://github.com/EriikGabriel/dev-environment-files/blob/{{TAG}}/docs/formatacao.md).
