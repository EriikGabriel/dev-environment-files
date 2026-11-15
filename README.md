# dev-environment-files

Centralizador das minhas configurações de ambiente. O `envsetup` detecta a máquina, lê uma
descrição declarativa do ambiente (apps, dotfiles e pastas) e aplica só o que falta. Tem um
app desktop (Tauri + React) e uma linha de comando sobre o mesmo motor em Go.

| Ambiente | Máquina | Estado |
|---|---|---|
| Desenvolvimento | CachyOS e Arch | niri + Noctalia, fish + starship, kitty, Zed |
| Desenvolvimento | Ubuntu e Debian | GNOME + WhiteSur, zsh + powerlevel10k, WezTerm, VS Code |
| Jogos | Windows | planejado; ainda sem etapas |

## App desktop

Mostra o sistema em uso, guia a configuração em etapas, deixa ler cada arquivo de config,
editar a estrutura de pastas, arrastar configs novas para o repo e acompanhar a aplicação
com log em tempo real. A aba **Preview** simula o desktop do perfil (wallpaper, painel, dock
com os ícones instalados) e permite reordenar a dock.

Requer Go, Node 20+, Rust e, no Linux, `webkit2gtk-4.1`.

```bash
cd app
npm install
npm run tauri dev      # desenvolvimento (compila o motor Go antes)
npm run tauri build    # gera o instalador
```

## Linha de comando

Requer Go e `sudo`. Rode como usuário normal: o `envsetup` chama `sudo` quando precisa.

```bash
go build -o envsetup ./cmd/envsetup

./envsetup info       # detalhes da máquina
./envsetup steps      # etapas do perfil e o estado de cada item
./envsetup plan       # o que mudaria, sem alterar nada
./envsetup apply      # aplica (pede confirmação; -y para pular)
./envsetup            # TUI simples, para SSH ou máquina sem ambiente gráfico

./envsetup apply --choose editor=vscode --choose shell=none   # troca ou pula uma escolha
./envsetup apply --apps git,docker                            # só esses itens marcáveis
./envsetup add ~/.config/algo                                 # guarda uma config no repo
./envsetup show dotfiles/kitty/kitty.conf                     # lê um arquivo do repo
./envsetup set-dirs --step workspace --option www ~/www/projects ~/www/games
./envsetup desktop                                            # o que o preview mostra
./envsetup set-dock kitty dev.zed.Zed GodotHub                # ordem da dock no perfil
```

`-p/--profile` escolhe o perfil; sem ele vale o que corresponde à máquina. `--root` aponta
para o repo quando o binário está fora dele. A maioria dos comandos aceita `--json`.

## Como a configuração é organizada

```
config/apps.yaml          catálogo de apps, com nome e método de instalação por distro
config/profiles/*.yaml    um perfil por tipo de máquina, dividido em etapas
dotfiles/                 os arquivos de configuração
app/                      app desktop (Tauri + React)
cmd/, internal/           motor e CLI em Go
legacy/                   scripts bash antigos
docs/decisions/           decisões de arquitetura
```

**Catálogo.** Cada campo por distro é uma tabela `{debian: ..., arch: ..., "*": ...}`, buscada
do ID exato (`ubuntu`, `cachyos`) para o `ID_LIKE` (`debian`, `arch`) e por fim `"*"`. Os
métodos são `native`, `aur`, `deb-url`, `flatpak` e `script`; o cabeçalho do arquivo explica
cada um.

**Perfis.** O perfil é escolhido por `match`. Cada um é uma lista de etapas (Essenciais,
Shell, Terminal e fontes, Linguagens e serviços, Editor, Aplicativos, Pastas de trabalho,
Desktop). Numa etapa:

- `apps` começam marcados e `optional` começam desmarcados;
- `options` é uma escolha única com `default` (`none` pula);
- `files` são dotfiles; com `app:` o arquivo só é implantado junto com aquele app;
- `dirs` são pastas a criar.

**Dotfiles.** São implantados por symlink: o destino na home aponta para o arquivo do repo, e
o que existia antes vira `*.envsetup-<timestamp>.bak`. Editar a config no lugar de sempre já
altera o repo.

**Desktop e dock.** A seção `desktop:` do perfil descreve wallpaper, painel, cores e a ordem
da dock. Ao aplicar, a ordem é escrita no desktop de verdade (Noctalia: `pinned` do
`config.toml`; GNOME: favoritos). Edite pela aba Preview ou direto no YAML.

**Aplicar.** A ordem é apps, pastas, arquivos e extensões do VS Code. É idempotente: o que já
está em dia é pulado (comandos `post` rodam sempre). Cada execução grava um log em
`~/.local/state/envsetup/apply-<data>.log`.

Para adicionar um app, crie a entrada em `config/apps.yaml` e ponha o id numa etapa do perfil.

## Segredos

Não faça commit de chaves ou senhas. Nas cópias guardadas aqui:

- `dotfiles/zed/settings.json` tem `<FIGMA_API_KEY>` no lugar da chave do Figma;
- `dotfiles/bruno/collections` tem `<PASSWORD>` no lugar das senhas das requisições.

Depois de aplicar, o arquivo da home é um symlink para o do repo: se você preencher o valor
real nele, o valor passa a estar dentro do repositório.

## Notas

- **Zed:** as extensões ficam em `auto_install_extensions` no `settings.json`; o Zed as instala
  sozinho.
- **GNOME (Ubuntu):** os itens da etapa Desktop falam com a sessão GNOME em execução, então só
  funcionam de dentro dela. O GNOME pede confirmação para cada extensão.
- **Ícones:** MacTahoe no CachyOS e WhiteSur no Ubuntu.
- **Wallpaper:** `dotfiles/wallpapers/macos-ventura.jpg` (3,6 MB) é implantado em
  `~/Imagens/Wallpapers`; no Ubuntu o preview baixa o wallpaper do perfil.
- **Bruno:** `dotfiles/bruno/workspace.yml` tem o caminho absoluto da home.
- **niri:** `dotfiles/niri/monitors.kdl` é específico dos monitores desta máquina.
- **Vicinae:** vem do AUR (`vicinae-bin`) no Arch; no Ubuntu é o AppImage, opcional. Só o
  `settings.json` vai para o repo; as extensões e o histórico em `~/.local/share/vicinae` ficam de fora.
- **GodotHub:** é um AppImage e precisa do FUSE 2.

## Desenvolvimento

```bash
go test ./...                    # motor
cd app && npx tsc --noEmit       # tipos do front-end
```

O contrato entre o motor e o app está em
[`docs/decisions/0002-tauri-react.md`](docs/decisions/0002-tauri-react.md); o modelo de perfis
e etapas em [`0003`](docs/decisions/0003-perfis-e-etapas.md).

## Legado

`legacy/` guarda os scripts bash originais (Ubuntu + GNOME) e o tema GTK embutido que o perfil
do Ubuntu ainda usa. O que eles faziam foi portado para o perfil `dev-ubuntu`, exceto o fluxo
de reinício com autostart, a identidade git, a chave SSH e o `gh auth login`.

## To-do

- [ ] Perfil de jogos no Windows (backends winget/scoop)
- [ ] Repositórios de terceiros declarativos (docker-ce, NodeSource)
- [ ] Identidade git, chave SSH e `gh auth login`
- [ ] `webkit2gtk` e FUSE 2 no catálogo
- [ ] Testar os scripts do GNOME numa sessão real
