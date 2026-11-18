# dev-environment-files

Centralizador das minhas configurações de ambiente. O `envsetup` detecta a máquina, lê uma
descrição declarativa do ambiente (apps, dotfiles e pastas) e aplica só o que falta. Tem um
app desktop (Tauri + React) e uma linha de comando sobre o mesmo motor em Go.

| Ambiente        | Máquina         | Estado                                                  |
| --------------- | --------------- | ------------------------------------------------------- |
| Desenvolvimento | CachyOS e Arch  | niri + Noctalia, fish + starship, kitty, Zed            |
| Desenvolvimento | Ubuntu e Debian | GNOME + WhiteSur, zsh + powerlevel10k, WezTerm, VS Code |
| Jogos           | Windows         | instalação automática (Ventoy), winget, Ninite, políticas |

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

# Windows (perfil de jogos)
./envsetup media                                  # o que o autounattend.xml faz, Ninite e pendrives Ventoy
./envsetup open-url unattend                      # abre o gerador com as opções do arquivo atual
./envsetup set-unattend ~/Downloads/autounattend.xml   # troca o arquivo pelo que o gerador baixou
./envsetup set-ninite 7zip steam discord          # lista do Ninite (ou um link https://ninite.com/...)
./envsetup write-media --drive D:                # grava XML, ventoy.json e Ninite no pendrive
./envsetup policies                               # políticas de grupo: escolha e estado de cada uma
./envsetup set-policy widgets=off                 # video (padrão), off ou uma opção da política
./envsetup apply-policies                         # grava no registro (pede elevação uma vez)
```

`-p/--profile` escolhe o perfil; sem ele vale o que corresponde à máquina. `--root` aponta
para o repo quando o binário está fora dele. A maioria dos comandos aceita `--json`.

## Como a configuração é organizada

```
config/apps.yaml          catálogo de apps, com nome e método de instalação por distro
config/profiles/*.yaml    um perfil por tipo de máquina, dividido em etapas
config/policies.yaml      políticas de grupo do Windows, com explicação e escolha de cada uma
windows/                  autounattend.xml e ventoy.json da instalação automática do Windows
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

## Windows: instalação automática, Ninite e políticas

O perfil `gaming-windows` tem uma seção `media:` e, no app, as abas **Instalação** e **Políticas**; a aba **Preview** mostra o desktop do Windows.

- **Arquivo de resposta.** `windows/autounattend.xml` foi feito em
  [schneegans.de](https://schneegans.de/windows/unattend-generator/) e traz, num comentário, o
  link que reabre o gerador com tudo o que foi marcado. O envsetup lê esse link para mostrar o
  que o arquivo faz (idioma, conta, ajustes, apps removidos, scripts) e para abrir o site já
  preenchido. Para mudar: abra o site, ajuste, baixe o XML e importe (`set-unattend`).
- **Ninite.** A lista de programas é o item `ninite` do catálogo (`names: { windows: ... }`).
  Marque os programas no app, ou monte a seleção em ninite.com e cole o link. O instalador é
  baixado na hora de `https://ninite.com/<lista>/ninite.exe`. O item `ninite` da etapa
  Essenciais roda esse instalador; ele conta como feito depois de rodar para aquela lista exata.
- **Pendrive Ventoy.** `write-media` copia o XML para o caminho indicado no `ventoy.json`,
  mescla o `ventoy.json` (o que só existe no pendrive é mantido; o que muda ganha um `.bak`) e
  baixa o Ninite para a **raiz** como `NiniteInstaller.exe`, que é onde o script de primeiro
  login do XML o procura (`Ninite*Installer.exe` na raiz de cada unidade). A imagem do Windows
  não é copiada.
- **Políticas de grupo.** `config/policies.yaml` lista as 42 configurações do gpedit feitas
  [neste vídeo](https://youtu.be/kQM-iv7TQz0?t=1617), com o que cada uma faz e as ressalvas. Cada
  uma tem uma escolha: `video` (padrão), `off` (não configurar) ou uma opção própria. O item
  `group-policies` da etapa Otimizações (desmarcado por padrão) e o botão da aba Políticas gravam
  os mesmos valores que o gpedit gravaria, no `Registry.pol` (que o gpedit lê) e no registro, e
  rodam `gpupdate`. Reabra o gpedit para ver as políticas como Habilitadas/Desabilitadas.
- **Preview e layout.** Como no perfil de dev, a aba Preview mostra o desktop: a barra de tarefas (com as
  opções do arquivo de resposta: alinhamento, pesquisa, widgets), o menu Iniciar e os ícones. A
  *bandeja de apps* reúne os apps da lista do Ninite, os que o arquivo de resposta instala por
  script (como o Hydra) e os marcados em Configurar; arraste-os para a barra e reordene. As
  escolhas ficam em `config/layout.yaml` (`set-layout`) e `apply-layout` as grava no Windows: a
  barra por um arquivo `TaskbarLayout.xml` apontado pela política "Start layout", o menu Iniciar
  limpo pela política `ConfigureStartPins` e os ícones da área de trabalho por `HideIcons`. Os
  atalhos são procurados no menu Iniciar, então um app só entra na barra depois de instalado.
  Uma barra vazia deixa a barra atual como está.
- **Ícones no estilo macOS.** O Windows não tem tema de ícones (o ícone de um `.exe` fica dentro
  do arquivo), então o envsetup troca o que dá: o ícone dos atalhos (`.lnk`) da área de trabalho,
  do menu Iniciar e da barra de tarefas, e, se o pack tiver, os ícones de Este Computador, Lixeira,
  Arquivos do usuário e Rede. Para cada atalho ele pega o ícone que o shell mostra (256 px) e usa o
  arquivo do icon pack que tiver o nome do app (`discord.png`, `epic-games.ico`...); quem não tem
  recebe o ícone original numa placa de cantos arredondados, clara ou escura. O pack que vem no
  repositório é o **MacTahoe** (`windows/icons/mactahoe`): os ~1400 ícones de apps do
  [MacTahoe-icon-theme](https://github.com/vinceliuice/MacTahoe-icon-theme), de vinceliuice
  (GPL-3.0, licença na pasta), já desenhados em PNG de 256 px, mais os outros nomes de cada ícone
  (`aliases.json`) e os ícones do sistema. Nada é baixado ao aplicar; `go run ./cmd/iconpack`
  refaz a pasta a partir do tema (usa o Edge para desenhar os SVGs). Programas do Windows com outro
  nome no tema (`cmd`, PowerShell, Git Bash, Gerenciador de Tarefas...) têm o ícone equivalente, e
  `icon-pack --query nome` lista os ícones do pack, que o app mostra numa galeria com busca. Fica em
  `config/icons.yaml` (`set-icons`); `icons --scan` mostra o antes e o depois, `apply-icons` aplica
  (uma elevação, e o Explorer reinicia) e `restore-icons` (no app, "Voltar aos ícones do Windows") desfaz tudo:
  devolve o ícone original de cada atalho, guardado em `%ProgramData%\envsetup\icons\manifest.json`,
  e o de qualquer outro atalho que ainda aponte para um ícone criado (um app fixado depois de
  aplicar, por exemplo), tira os ícones do sistema, apaga os ícones criados e refaz o cache de
  ícones do Windows. Atalhos sem alvo `.exe` (os do Office
  instalados pelo MSI, por exemplo) e links de desinstalar/ajuda ficam como estão. Não muda o ícone
  de um `.exe` no Explorer nem o de janelas abertas, e um app que recria o atalho ao atualizar
  (Discord, Steam) volta ao ícone dele até você aplicar de novo.
- **Dock na barra de tarefas (Windhawk).** A dock é a do mod "Ilha e Dock estilo Mac"
  (`mac-island-dock`). `envsetup taskbar-dock` põe a opção "Barra de tarefas" dele em "Dock" pelo
  `windhawk-cli` (instala o mod se ele faltar; uma elevação, porque o Windhawk guarda as
  configurações em HKLM) e lembra o modo anterior; `--restore` o devolve. No app: "Ligar dock" e
  "Desligar dock". Tamanho, ampliação e animações são configurações do mod e ficam no backup do
  Windhawk.
- **Mods do Windhawk.** O Windhawk 2 exporta num arquivo só as configurações dele, os mods
  instalados e as configurações de cada mod; o repo guarda esse arquivo em
  `windows/windhawk/backup.json`. `envsetup windhawk` compara o arquivo com o que está instalado
  (mod faltando, versão, ativado/desativado, configurações diferentes), `windhawk-import` aplica
  com o `windhawk-cli` (instala os mods do repositório do Windhawk e grava as configurações; uma
  elevação) e `windhawk-export` troca o arquivo pelo que está na máquina. No perfil de jogos, a
  etapa **Aparência** instala o Windhawk e aplica o backup; a aba **Windhawk** do app mostra cada
  mod e as configurações dele.
- **Playnite portable por snapshot.** O Playnite portable guarda tudo na própria pasta (biblioteca,
  capas, trailers, músicas de tema, temas, extensões e as configurações delas), então uma cópia da
  pasta é o setup inteiro. Ela tem dezenas de GB e não vai para o git: `config/playnite.yaml` diz
  onde está a pasta (`path`, padrão `C:\Playnite`) e onde fica o snapshot (`store`: HD externo, NAS,
  pasta sincronizada). `playnite-save` espelha a pasta em `<store>\Playnite` com o robocopy (só o que
  mudou; só ficam de fora `JITProfiles` e os logs, então capas, trailers, músicas e os logins das lojas
  vão junto) e grava um manifesto ao lado;
  `playnite-restore` copia de volta, corrige os caminhos fixos das extensões (ffmpeg, yt-dlp) se a
  pasta mudou e registra o Playnite no menu Iniciar (`windows/register-portable-app.ps1`). Os dois
  recusam rodar com o Playnite aberto. No perfil de jogos, a etapa **Playnite** faz a restauração. A
  aba **Playnite** do app mostra temas, extensões, o tamanho de cada pasta (dá para tirar uma do
  snapshot) e o que a pasta não leva, com botão para corrigir: o **Windows Media Player**, que o
  Playnite usa para tocar trailers (o `autounattend.xml` não o remove mais, e o item
  `windows-media-player` da etapa Essenciais o reinstala em máquinas antigas), o atalho no menu
  Iniciar e programas fora da pasta, como o `ffmpeg-normalize` do Playnite Sounds Mod.

## Segredos

Não faça commit de chaves ou senhas. Nas cópias guardadas aqui:

- `dotfiles/zed/settings.json` tem `<FIGMA_API_KEY>` no lugar da chave do Figma;
- `dotfiles/bruno/collections` tem `<PASSWORD>` no lugar das senhas das requisições;
- `windows/autounattend.xml` tem o nome da conta local e nenhuma senha. Se você puser uma senha
  no gerador, ela fica legível no XML e no link (a "ofuscação" é só Base64): o app avisa ao
  importar, e esse arquivo não deve ir para um repositório público.

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

- [x] Perfil de jogos no Windows (backends winget/scoop)
- [x] Repositórios de terceiros declarativos (docker-ce, NodeSource)
- [x] Identidade git, chave SSH e `gh auth login`
- [x] `webkit2gtk` e FUSE 2 no catálogo
- [ ] Configurar Playnite (o snapshot já está pronto: falta escolher o `store` e salvar)
- [ ] Emulador (?)
- [ ] Terminar de configurar video do ET
- [ ] Configurar aparencia geral com alguns tweaks temas e icones (pra dps jogar pro app)
- [ ] Testar os scripts do GNOME numa sessão real
