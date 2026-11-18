# Como usar

## Instalar

**Pelo instalador** (o caso normal). O instalador do app traz tudo o que ele precisa: o motor
(`envsetup`, já compilado) e os dados (`config/`, `dotfiles/`, `windows/`). Não é preciso ter Go,
Node, Rust nem git. No Windows o app usa o WebView2, que já vem no Windows 11; no Linux precisa do
`webkit2gtk-4.1`.

**Por um clone** (para versionar as mudanças com git):

```bash
git clone https://github.com/EriikGabriel/dev-environment-files
```

Depois abra o app e escolha a pasta do clone, ou use a linha de comando (abaixo).

## A pasta de dados

O envsetup lê e grava numa **pasta de dados**, com `config/`, `dotfiles/` e `windows/`. Na
primeira execução o app pergunta qual usar:

- **"Usar os dados incluídos"**: copia os dados do instalador para `Documentos\envsetup`
  (Windows) ou `~/envsetup` (Linux) e passa a usar essa cópia. É nela que você edita.
- **"Escolher pasta…"**: um clone do repositório (as mudanças vão para o git) ou qualquer pasta
  com `config/apps.yaml`.

A escolha fica salva; troque pelo botão "Trocar pasta de dados…" na barra lateral. Quando uma
versão nova do app traz arquivos novos (perfis, ícones, scripts), "Atualizar dados" copia para a
sua pasta **só o que ainda não existe nela**; nada que você editou é sobrescrito. Num clone git o
botão não aparece: atualize com `git pull`.

> Os dotfiles são implantados como symlink para a pasta de dados. Se você mover ou apagar a
> pasta, os links quebram: aponte o app para o novo lugar e aplique de novo.

## O app

| Aba | Para quê |
|---|---|
| **Início** | o sistema detectado, o perfil, o que está pendente e um botão para aplicar |
| **Configurar** | as etapas do perfil: marcar apps, escolher opções, adicionar apps, editar pastas; no fim, revisar e aplicar |
| **Preview** | no Linux, o desktop do perfil (wallpaper, painel, dock reordenável); no Windows, a barra de tarefas e o menu Iniciar, o cartão de ícones e a dock do Windhawk |
| **Dotfiles** | os arquivos que o perfil implanta, o estado de cada um e "Adicionar" para trazer uma config da máquina |
| **Instalação** (Windows) | o arquivo de resposta, a lista do Ninite e a gravação do pendrive Ventoy |
| **Políticas** (Windows) | as políticas de grupo, a escolha de cada uma e "Aplicar" |
| **Windhawk** (Windows) | os mods do backup comparados com os instalados; aplicar e salvar |
| **Playnite** (Windows) | o snapshot do Playnite portable e o que ele precisa fora da pasta |

**Aplicar** é idempotente: o que já está em dia é pulado e cada passo mostra o log ao vivo. No
Linux o app pede a senha do sudo uma vez; no Windows cada instalador pede a própria elevação
(UAC). Cada execução grava um log em `~/.local/state/envsetup/apply-<data>.log` (Linux) ou
`%LOCALAPPDATA%\envsetup\apply-<data>.log` (Windows).

O que muda cada arquivo de configuração, e como editar, está em
[configuracao.md](configuracao.md).

## Linha de comando

O app usa o mesmo programa, `envsetup`, com `--json`. Ele também roda sozinho: no instalador do
Windows fica na pasta do app (`envsetup.exe`); num clone, compile com
`go build -o envsetup ./cmd/envsetup`. No Linux rode como usuário normal: ele chama o `sudo`
quando precisa.

A pasta de dados é achada nesta ordem: `--root`, `$ENVSETUP_ROOT`, uma pasta acima da atual (um
clone) e por fim `Documentos\envsetup` / `~/envsetup`. `-p` escolhe o perfil (padrão: o que bate
com a máquina). `envsetup <comando> --help` mostra as opções de cada um.

```bash
# a máquina e o perfil
envsetup info                      # detalhes da máquina
envsetup profiles                  # perfis disponíveis
envsetup steps                     # etapas do perfil e o estado de cada item

# planejar e aplicar
envsetup plan                      # o que mudaria, sem alterar nada
envsetup apply                     # aplica (pede confirmação; -y para pular)
envsetup apply --choose editor=vscode --choose shell=none   # troca ou pula uma escolha
envsetup apply --apps git,docker   # só esses itens
envsetup apply --no-apps           # só os dotfiles

# editar o perfil e os dotfiles
envsetup add ~/.config/algo                     # guarda uma config na pasta de dados
envsetup show dotfiles/kitty/kitty.conf         # lê um arquivo da pasta de dados
envsetup add-app --step base --id htop --name htop
envsetup remove-app --step base htop
envsetup set-dirs --step workspace --option www ~/www/projects ~/www/games
envsetup set-dock kitty dev.zed.Zed GodotHub    # ordem da dock (Linux)

# Windows: instalação automática
envsetup media                                  # o que o autounattend.xml faz, Ninite e pendrives
envsetup open-url unattend                      # abre o gerador com as opções do arquivo atual
envsetup set-unattend ~/Downloads/autounattend.xml
envsetup set-ninite 7zip steam discord          # ou um link https://ninite.com/...
envsetup write-media --drive D:

# Windows: sistema e aparência
envsetup policies | set-policy widgets=off | apply-policies
envsetup layout   | set-layout --taskbar=explorer,steam | apply-layout
envsetup icons --scan | set-icons --style=dark | apply-icons | restore-icons
envsetup windhawk | windhawk-export | windhawk-import
envsetup taskbar-dock [--restore]
envsetup playnite --sizes | set-playnite --store=E:\Backups | playnite-save | playnite-restore
envsetup playnite-fix media-player
```

## Windows em detalhe

- **Arquivo de resposta.** `windows/autounattend.xml` instala o Windows sem perguntas: idioma,
  conta, ajustes, apps removidos e scripts de primeiro login. Ele é feito no gerador do
  schneegans.de e guarda o link que reabre o site com as mesmas opções; o app usa esse link para
  resumir o arquivo e para abrir o site preenchido.
- **Ninite.** Um instalador só para a lista de programas do item `ninite`. Ele é baixado de
  `https://ninite.com/<lista>/ninite.exe`; no pendrive vai para a raiz como `NiniteInstaller.exe`,
  onde o script de primeiro login o procura. O item conta como feito depois de rodar para aquela
  lista exata.
- **Pendrive Ventoy.** "Gravar neste pendrive" copia o XML para o caminho do `ventoy.json`, mescla
  o `ventoy.json` com o do pendrive (o que muda ganha um `.bak`) e baixa o Ninite. A imagem do
  Windows não é copiada.
- **Políticas.** Gravadas no `Registry.pol` (que o gpedit lê) e no registro, seguido de
  `gpupdate`. Algumas só valem depois de sair da sessão ou reiniciar.
- **Barra de tarefas e menu Iniciar.** A barra é escrita num `TaskbarLayout.xml` apontado pela
  política "Start layout"; o menu Iniciar é limpo por `ConfigureStartPins` e os ícones da área de
  trabalho por `HideIcons`.
- **Ícones.** O Windows não tem tema de ícones; o envsetup troca o ícone dos atalhos (área de
  trabalho, menu Iniciar, barra) e, se o pack tiver, os de Este Computador, Lixeira, Usuário e
  Rede. Um app com arquivo no pack recebe esse arquivo; os outros recebem o ícone original numa
  placa arredondada. O ícone original de cada atalho fica em
  `%ProgramData%\envsetup\icons\manifest.json` para desfazer. Apps que recriam o atalho ao
  atualizar (Discord, Steam) voltam ao ícone deles até você aplicar de novo. O pack MacTahoe vem
  do [MacTahoe-icon-theme](https://github.com/vinceliuice/MacTahoe-icon-theme) (GPL-3.0);
  `go run ./cmd/iconpack` o refaz.
- **Windhawk.** Precisa do Windhawk 2 (o 1.x não tem o `windhawk-cli`). Aplicar baixa os mods do
  repositório do Windhawk, então precisa de internet. Mods que só existem na máquina ficam como
  estão. A dock é a do mod "Ilha e Dock estilo Mac"; ligar instala o mod se faltar e lembra o modo
  anterior da barra para desligar.
- **Playnite.** Salvar e restaurar recusam rodar com o Playnite aberto. Restaurar não apaga o
  que já está na pasta, a não ser com `--mirror`. O Windows Media Player, que toca os trailers,
  vem no item `windows-media-player` da etapa Essenciais (o `autounattend.xml` não o remove mais).

## Notas do Linux

- **Zed:** as extensões ficam em `auto_install_extensions` no `settings.json`; o Zed as instala.
- **GNOME (Ubuntu):** os itens da etapa Desktop falam com a sessão GNOME em execução, então só
  funcionam de dentro dela. O GNOME pede confirmação para cada extensão.
- **Ícones:** MacTahoe no CachyOS e WhiteSur no Ubuntu.
- **Wallpaper:** `dotfiles/wallpapers/macos-ventura.jpg` vai para `~/Imagens/Wallpapers`; no
  Ubuntu o preview baixa o wallpaper do perfil.
- **Vicinae:** vem do AUR (`vicinae-bin`) no Arch; no Ubuntu é o AppImage, opcional. Só o
  `settings.json` é guardado; extensões e histórico ficam de fora.
- **GodotHub:** é um AppImage e precisa do FUSE 2.
