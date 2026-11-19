# Configuração: o que cada arquivo faz e como mudar

Tudo o que o envsetup sabe fazer vem da **pasta de dados**: um clone deste repositório ou a cópia
que o app faz na primeira execução (`Documentos\envsetup` no Windows, `~/envsetup` no Linux; veja
[uso.md](uso.md#a-pasta-de-dados)). Nada fica embutido no programa: mudar o ambiente é mudar
arquivos desta pasta.

```
config/apps.yaml           catálogo: como instalar cada app em cada sistema
config/profiles/*.yaml     um perfil por máquina: as etapas e o que cada uma instala
config/policies.yaml       políticas de grupo do Windows
config/layout.yaml         barra de tarefas, menu Iniciar e ícones da área de trabalho (Windows)
config/icons.yaml          estilo dos ícones dos atalhos (Windows)
config/playnite.yaml       onde fica o Playnite portable e o HD com a biblioteca e as mídias dele
config/retroarch.yaml      RetroArch portable: versão, cores, pastas e o HD com a BIOS e os jogos
windows/playnite/          as configurações do Playnite e das extensões (sem chaves de API)
windows/retroarch/         retroarch.cfg, opções dos cores, playlists, saves e estados (sem senhas)
windows/autounattend.xml   instalação automática do Windows (com windows/ventoy.json)
windows/windhawk/backup.json   mods do Windhawk e as configurações deles
windows/icons/mactahoe/    pack de ícones (PNG por nome de app)
dotfiles/                  os arquivos de configuração que vão para a home
```

**Três jeitos de mudar uma config.** Pela aba do app que cuida dela, por um comando
(`envsetup set-…`) ou editando o arquivo. Os arquivos da `config/` começam com um comentário que
explica cada campo, e esse comentário é a referência; este guia é o mapa. O app e os comandos
`set-…` reescrevem só o que mudou, então comentários seus nos arquivos editados à mão continuam
lá, exceto em `layout.yaml`, `icons.yaml`, `playnite.yaml` e `retroarch.yaml`, que o envsetup regrava inteiros (com o
cabeçalho padrão). Depois de editar à mão, abra o app ou rode `envsetup plan`: um arquivo que não
carrega mostra o erro com a linha.

---

## `config/apps.yaml`: o catálogo de apps

Cada app tem um id e diz, por sistema, **como** instalar. Um app só é instalado quando algum
perfil o põe numa etapa.

```yaml
apps:
  ripgrep: { names: { "*": ripgrep } }          # mesmo nome de pacote em todo Linux
  steam: { description: Steam, names: { windows: Valve.Steam } }   # ID do winget
  docker:
    description: Docker engine
    names:
      debian: docker-ce docker-ce-cli containerd.io   # vários pacotes, separados por espaço
      arch: docker
    post:
      "*": ["$SUDO systemctl enable --now docker"]   # roda em todo apply: deve ser idempotente
```

**Por sistema.** Os campos `names`, `method`, `source`, `check`, `post` e `repo` são tabelas por
chave. A busca vai do ID exato (`ubuntu`, `cachyos`) para a família (`debian`, `arch`) e por fim
`"*"`, que vale para todo **Linux**. O Windows só lê a chave `windows`.

**Métodos** (`method`, padrão `native`):

| Método | Onde | `names` é |
|---|---|---|
| `native` | apt, pacman, winget | o pacote (ou o ID do winget) |
| `aur` | Arch | o pacote do AUR (via yay ou paru) |
| `deb-url` | Debian/Ubuntu | o nome do pacote; `source` é a URL do `.deb` |
| `flatpak` | Linux | o ID do Flathub |
| `script` | todos | não usa; `source` é o script e `check` sai com 0 quando já está instalado |
| `winget`, `scoop` | Windows | o ID exato |
| `steam` | Windows | o AppID (o app precisa estar na conta) |
| `ninite` | Windows | a lista de programas do ninite.com |
| `policies` | Windows | não usa: aplica `config/policies.yaml` |
| `playnite` | Windows | não usa: restaura o snapshot de `config/playnite.yaml` |
| `windhawk` | Windows | não usa: importa `windows/windhawk/backup.json` |

**Scripts.** `source`, `check` e `post` rodam com `sh` no Linux (com `$SUDO` valendo `sudo`) e
com PowerShell no Windows. Eles recebem `$ENVSETUP_ROOT` (a pasta de dados) e um
`$ENVSETUP_<CHAVE>` para cada valor que uma etapa pede (veja `inputs` abaixo). `{arch}`, `{x64}` e
`{codename}` em `source` viram a arquitetura e o codinome da versão.

**Outros campos.** `unavailable: [debian]` esconde o app nesses sistemas; `repo` (Debian/Ubuntu)
registra um repositório apt de terceiros antes de instalar (`key` é a URL da chave e `source` a
parte `url suíte componentes` da linha do sources.list).

**Software da placa de vídeo (`gpu`).** `gpu: [nvidia]` (ou `amd`, `intel`) diz que o app é para
aquele tipo de placa. O envsetup lê as placas da máquina (`Win32_VideoController`): se nenhuma é
do tipo, o app vem **desmarcado** e o assistente mostra o motivo ("para placas AMD; esta máquina
tem NVIDIA"); ainda dá para marcar. Sem placa detectada, vem marcado. O perfil de jogos lista
`nvidia-app` e `amd-software` na etapa Drivers, e só o da placa da máquina vem marcado.

**Programas baixados do site do fabricante.** Para o que não está no winget, use o método
`script` com os dois ajudantes de `windows/`:

```yaml
  mchose-hub:
    method: { windows: script }
    check:
      windows: if (-not (& "$env:ENVSETUP_ROOT\windows\installed.ps1" 'MCHOSE HUB*')) { exit 1 }
    source:
      windows: |
        & "$env:ENVSETUP_ROOT\windows\install-from-url.ps1" -Name 'MCHOSE HUB*' -Url 'https://…/installer.zip'
```

`installed.ps1` procura o nome (com curingas) em Aplicativos instalados. `install-from-url.ps1`
baixa o instalador (ou um `.zip` com ele; `-Page` e `-Pattern` acham o link numa página, quando
ele muda a cada versão), tenta instalar sem perguntas (`-Silent`, padrão `/S` dos instaladores
NSIS) e, se o programa não aparecer em Aplicativos instalados, abre o instalador para seguir à
mão. Assim são instalados o NVIDIA App, o MCHOSE HUB e o Attack Shark Driver.

**Para adicionar um app:** pelo app, no botão "+ Adicionar app" da etapa em Configurar (pede o nome do
pacote ou o ID do winget e cria a entrada), ou com
`envsetup add-app --step <etapa> --id <id> --name <pacote>`. À mão: crie a entrada aqui e ponha o
id numa etapa do perfil.

## `config/profiles/*.yaml`: os perfis

Um perfil descreve o ambiente final de um tipo de máquina. O app escolhe o perfil cujo `match`
bate com o sistema (`-p` escolhe outro).

```yaml
name: Dev · CachyOS (niri + Noctalia)
description: O ambiente atual - niri com Noctalia, fish + starship, kitty e Zed.
environment: dev          # dev ou gaming: só muda a cor e o rótulo no app
match: [cachyos, arch]    # IDs do sistema (os-release) ou "windows"
desktop: { ... }          # só Linux: o que a aba Preview desenha (veja abaixo)
media: { ... }            # só Windows: a instalação automática (veja autounattend)
steps: [ ... ]
```

**Etapas.** Cada etapa é uma tela de Configurar.

```yaml
  - id: shell
    title: Shell
    description: O shell de login e os utilitários.
    apps: [eza, bat]            # começam marcados
    optional: [git-delta]       # começam desmarcados
    choice_title: Shell         # escolha única entre `options`
    default: fish               # "none" pula a escolha
    options:
      - id: fish
        title: fish + starship
        apps: [fish, starship]
        files:
          - { src: dotfiles/fish/config.fish, dest: ~/.config/fish/config.fish }
    files:
      - { src: dotfiles/fastfetch/cachyos.jsonc, dest: ~/.config/fastfetch/config.jsonc, app: fastfetch }
    dirs: [~/www/projects]      # pastas a criar
    inputs:                     # valores que a etapa pede; scripts leem $ENVSETUP_GIT_NAME
      - { id: git.name, label: Nome no git, from: git config --global user.name }
```

- `files`: `src` é relativo à pasta de dados e `dest` aceita `~`. O padrão é **symlink** no Linux
  (editar a config no lugar de sempre altera a pasta de dados) e **cópia** no Windows. `mode: copy`
  força cópia; `app:` só implanta junto com aquele app; `only: [arch]` limita a sistemas.
- Uma `option` tem `apps`, `files`, `dirs` e `vscode_extensions` (arquivo com a lista).
- `inputs.from` é um comando cuja saída vira o valor sugerido.

**Pelo app:** Configurar mostra as etapas; dá para marcar itens, adicionar e remover apps
(`add-app`, `remove-app`) e editar as pastas (`set-dirs`). A ordem das etapas, os textos e as
opções se mudam no arquivo.

**`desktop:` (Linux).** `kind` (`niri-noctalia` ou `gnome`), `wallpaper`, `icon_theme`, `colors`,
`bar` e `dock` (`position`, `autohide`, `pinned`). A aba Preview desenha isso; `dock.pinned` é
gravado no desktop de verdade ao aplicar (Noctalia: `config.toml`; GNOME: favoritos). A ordem da
dock se muda arrastando na Preview (`envsetup set-dock`).

## `config/policies.yaml`: políticas de grupo (Windows)

As 42 configurações do gpedit do vídeo citado em `source`, cada uma com `path` (a pasta no
gpedit), `video` (o estado do vídeo), `about`, `caution` (ressalvas), os valores do registro em
`set` e a **escolha** em `choice`:

- `video`: o estado do vídeo (padrão, "Recomendado" no app);
- `off`: não configurar (os valores são apagados);
- o id de uma `option` da política (políticas liga/desliga ganham a opção oposta sozinhas).

**Mudar:** na aba Políticas, ou `envsetup set-policy widgets=off cortana=video`. Só a linha
`choice:` é reescrita. **Aplicar:** botão da aba, `envsetup apply-policies` ou o item
`group-policies` da etapa Otimizações (pede elevação). Chaves fora de
`HKLM\SOFTWARE\Policies` e `...\CurrentVersion\Policies` são recusadas.

## `config/layout.yaml`: barra de tarefas e área de trabalho (Windows)

```yaml
taskbar: [explorer, brave, steam, discord]   # apps fixados, da esquerda para a direita
clear_start: true                            # tira os apps fixados do menu Iniciar
hide_desktop_icons: false
```

Os ids são os que a aba Preview mostra na "bandeja de apps" (os da lista do Ninite, os que o
`autounattend.xml` instala e os marcados em Configurar). Monte arrastando na Preview
(`envsetup set-layout`) e aplique com o botão ou `envsetup apply-layout`. Uma lista vazia deixa a
barra como está; um app só entra na barra depois de instalado.

## `config/icons.yaml`: ícones no estilo macOS (Windows)

| Campo | O que faz |
|---|---|
| `enabled` | `apply-icons` não faz nada enquanto for `false` |
| `style` | `light` ou `dark`: a placa dos ícones desenhados |
| `pack` | pasta de `.png`/`.ico` com o nome dos apps (relativa à pasta de dados); o padrão é `windows/icons/mactahoe` |
| `scopes` | `desktop`, `start-menu`, `taskbar` |
| `system_icons` | também troca Este Computador, Lixeira, Usuário e Rede |
| `skip` | atalhos que mantêm o ícone |
| `overrides` | `nome do atalho: arquivo` para escolher a imagem de um app |

Tudo isso se muda no cartão Ícones da aba Preview: "Escanear atalhos" mostra o antes e o depois,
"Aplicar ícones" grava (uma elevação; o Explorer reinicia) e "Voltar aos ícones do Windows" desfaz.
Imagens escolhidas dentro da pasta de dados são guardadas com caminho relativo, então continuam
valendo em outra máquina. Para trocar o ícone de um app sem override, ponha um arquivo com o
nome dele no pack (`discord.png`); `envsetup icon-pack --query nome` mostra o nome que o pack usa.

## Onde fica cada parte dos programas portáteis

Playnite e RetroArch são portáteis: tudo deles mora na própria pasta. Cada parte vai para onde
pode ir:

| Parte | Onde | Por quê |
|---|---|---|
| configurações, saves, playlists (poucos MB) | **git**, em `windows/playnite/` e `windows/retroarch/` | pequenas, mudam à mão, ganham histórico |
| senhas, chaves de API, tokens | **HD**, em `<store>\envsetup-<programa>-secrets.json` | o repositório é público |
| programa RetroArch, assets e cores | **download** de buildbot.libretro.com | são públicos, na versão gravada no yaml |
| jogos e BIOS | **HD** | direitos autorais e passam do limite do GitHub |
| biblioteca, capas, trailers, temas e extensões do Playnite | **HD** | dezenas de GB, demoram a baixar de novo |
| caches (`cache`, `browsercache`, logs) | **em lugar nenhum** | o programa refaz sozinho |

**Salvar** (o botão das abas, ou `playnite-save` / `retroarch-save`) faz as duas coisas: copia a
parte pessoal para `windows/…` e o resto para o HD (só o que mudou). Depois, faça commit e push.
Sem HD escolhido, só o git é atualizado.

**Senhas fora do git.** Antes de copiar para o git, o envsetup apaga o valor das configurações
cujo nome indica segredo (`password`, `api_key`, `access_token`, `refresh_token`, `token`,
`secret`, `cookies`) nos arquivos `.json`, `.cfg` e `.opt`. Os valores vão para o arquivo de
segredos do HD, e a restauração os devolve. Sem o HD, a restauração funciona e o programa pede
os logins de novo. Datas de expiração, tipos e atalhos (`TokenExpiryUtc`, `TokenType`,
`…Hotkey`) não são segredos e ficam.

**A letra do HD mudou?** Se o `store` não existe mas o mesmo caminho existe em outra letra com o
manifesto dentro (`E:\envsetup` virou `F:\envsetup`), o envsetup usa esse.

## `config/playnite.yaml`: Playnite portable (Windows)

| Campo | O que faz |
|---|---|
| `path` | a pasta do Playnite (padrão `C:\Playnite`) |
| `store` | o HD: a cópia vai para `<store>\Playnite`, com o manifesto `<store>\envsetup-playnite.json` e as chaves tiradas do git |
| `exclude` | o que nunca é copiado (padrão: `cache`, `browsercache`, `JITProfiles`, `*.log`): nomes com `*` ou ponto são padrões de arquivo; o resto, pastas relativas a `path` |
| `start_menu` | depois de restaurar, põe o Playnite no menu Iniciar e em Configurações › Apps |
| `git` | o que vai para `windows/playnite` (`*` vale uma pasta; `!` no começo tira um arquivo) |
| `version` | a versão do Playnite baixada quando não há HD; gravada ao salvar |

### O que o GitHub garante do Playnite, e o que só o HD guarda

Ao salvar, o envsetup grava no git (`windows/playnite`) tudo o que é pequeno e define *como* o
seu Playnite é. Assim ele volta mesmo sem o HD:

| No GitHub | Arquivo | O que volta com ele |
|---|---|---|
| A versão do programa | `version` em `config/playnite.yaml` | o Playnite portable é baixado dos releases oficiais |
| A lista de temas e extensões | `windows/playnite/addons.yaml` | cada um é baixado do banco de add-ons do Playnite (api.playnite.link), na versão que você usava quando ela ainda existe |
| As configurações do Playnite | `config.json`, `fullscreenConfig.json` | tema escolhido, layout, filtros, atalhos, modo tela cheia |
| As configurações de cada extensão | `ExtensionsData/*/config.json` | tudo o que você ajustou nas extensões, **sem** chaves de API e tokens (ficam no HD) |
| A estrutura da biblioteca | `library/*.db`, menos `games.db` | plataformas, **emuladores (o RetroArch)**, as **pastas escaneadas** de ROMs, presets de filtro, tags, categorias, status de conclusão |

E fica **só no HD**, porque não faz sentido no GitHub:

| Só no HD | Por quê | Sem o HD |
|---|---|---|
| `library/games.db`: a lista de jogos com tempo jogado, favoritos, notas, categorias de cada jogo, ocultos | muda a cada sessão e tem MBs binários: encheria o histórico do repositório | os jogos voltam pelas extensões de biblioteca (Steam, Epic, GOG…) e pelos scanners de ROM, mas **sem** tempo jogado, favoritos e notas |
| `library/files`: capas, ícones e fundos (~700 MB) | grande e baixável | os metadados são baixados de novo na importação |
| `ExtraMetadata`: trailers e logos (~22 GB) | enorme e lento de baixar | baixe de novo pelo Extra Metadata Loader |
| `ExtensionsData` além dos `config.json`: conquistas, cache das extensões (~2 GB) | dados que as extensões refazem | refeitos pelas extensões (as conquistas sincronizam de novo) |
| chaves de API e tokens | o repositório é público | entre de novo nas extensões (SteamGridDB, metadados por IA…) |
| temas e extensões que não estão no banco de add-ons (hoje: **PS5reborn**) | não há de onde baixar | instale à mão; a restauração diz quais |

**Com o HD**, a restauração copia tudo do HD e põe as configurações do git por cima; as bases da
biblioteca vêm do HD (não do git), para combinar com o `games.db` dele. **Sem o HD**, o Playnite
é baixado, os add-ons são instalados do banco e o git traz as configurações e a estrutura da
biblioteca. Leva uns 2 minutos. A aba também mostra o que a pasta não leva (Windows Media Player
para os trailers, programas externos) e corrige.

## `config/retroarch.yaml`: RetroArch portable e jogos (Windows)

| Campo | O que faz |
|---|---|
| `path` | a pasta do RetroArch, a que tem `retroarch.exe` (padrão `C:\Emuladores\RetroArch`) |
| `games` | a pasta dos jogos, uma subpasta por sistema (padrão `D:\Games\Retro`, no HD); vazio deixa os jogos fora. Para trocar, use "Trocar" na aba (move os arquivos) ou `retroarch-move-games` |
| `store` | o HD: `<store>\RetroArch` (a BIOS e o que não é baixado nem vai pro git), `<store>\Games`, o manifesto e as senhas tiradas do git |
| `exclude` | o que nunca é copiado (padrão: `logs`, `downloads`, `*.log`) |
| `start_menu` | depois de restaurar, põe o RetroArch no menu Iniciar e em Configurações › Apps |
| `version` | a versão que a restauração baixa; gravada ao salvar (`retroarch.exe --version`) |
| `cores` | os cores que a restauração baixa (nome do arquivo sem `_libretro.dll`); gravados ao salvar |
| `bundles` | os pacotes de assets baixados com o programa (`assets`, `autoconfig`, `cheats`, `database-rdb`, `info`, `overlays`, `shaders_slang`, `shaders_glsl`; também `database-cursors`, `shaders_cg`) |
| `git` | o que vai para `windows/retroarch` (padrão: `retroarch.cfg`, `retroarch-core-options.cfg`, `config`, `remaps`, `playlists`, `saves`, `states`, `thumbnails`) |
| `copy_program` | guarda também o programa inteiro no HD, para restaurar sem internet (~1 GB a mais) |
| `copy_games` | copia também os jogos para `<store>\Games` a cada salvamento. Desligado quando os jogos já moram num disco que não é formatado (`D:\Games\Retro`); ligue se o `store` for outro lugar (pendrive, NAS) e você quiser os jogos lá também |

**Por que os jogos não vão para o GitHub.** ROMs e BIOS têm direitos autorais, o repositório é
público e uma imagem de disco sozinha (o `.bin` de PlayStation) passa do limite de 100 MB do
GitHub. O `.gitignore` recusa as extensões comuns de ROM para evitar um commit por engano. Eles
ficam no HD (pode ser a mesma pasta de `store` do Playnite; cada um usa subpastas próprias).

**O que vai para cada lugar.** O RetroArch portable grava os caminhos relativos à pasta dele
(`libretro_directory = ":\cores"`). O git leva a parte pessoal; o programa (exe, DLLs, `assets`,
`autoconfig`, `cheats`, `database`, `filters`, `info`, `overlays`, `shaders`, `cores`) é
baixado; o HD fica com a `system/` (BIOS), screenshots e o que mais não for nenhum dos dois. Mude
as configurações pelo menu do próprio RetroArch e depois clique em Salvar.

**Restaurar** (etapa Emuladores do perfil, botão da aba ou `retroarch-restore`): se falta o
`retroarch.exe`, baixa o RetroArch de `version` e os `bundles`; copia o HD (BIOS e jogos); baixa
os `cores` que faltarem; põe os arquivos do git com as senhas do HD; e aponta as playlists para
os jogos. Leva uns 3 minutos, ~500 MB de download. Sem o HD, tudo menos jogos e BIOS volta.

**Playlists.** Os arquivos `.lpl` guardam o caminho completo de cada jogo e core. Quando as pastas
mudam de lugar, o RetroArch lista o jogo mas não o abre. A restauração (e o botão "Corrigir" da
aba, ou `envsetup retroarch-fix playlists`) procura cada jogo pelo nome do arquivo na pasta de
jogos, e cada core na pasta `cores`, e troca só os caminhos que não existem. Um nome que aparece
duas vezes na pasta de jogos fica como está.

## `windows/autounattend.xml` e `windows/ventoy.json`: instalação automática

O arquivo de resposta é feito no
[gerador do schneegans.de](https://schneegans.de/windows/unattend-generator/) e **não se edita à
mão**: ele guarda num comentário o link com todas as opções marcadas. Para mudar:

1. aba Instalação › "Customizar no site" (abre o gerador já preenchido; `envsetup open-url unattend`);
2. ajuste e baixe o XML;
3. "Importar XML…" na aba (`envsetup set-unattend <arquivo>`).

O app resume o que o arquivo faz e avisa de escolhas que quebram algo do setup (remover o Windows
Media Player, por exemplo). `ventoy.json` liga o XML à imagem do Windows no pendrive. O perfil
aponta para os dois em `media:` (`unattend`, `ventoy` e `ninite`, o id do item de catálogo cuja
lista do Ninite vai para o pendrive). "Gravar neste pendrive" (`envsetup write-media --drive D:`)
copia tudo para um pendrive Ventoy.

**Ninite.** A lista é o `names.windows` do item `ninite` do catálogo. Mude marcando os
programas na aba Instalação e clicando em "Salvar lista", colando um link do ninite.com ("Usar este link") ou com `envsetup set-ninite 7zip steam`.

## `windows/windhawk/backup.json`: mods do Windhawk

É o arquivo que o **Windhawk 2** exporta (configurações dele, mods instalados e as configurações
de cada mod). Não edite à mão: configure os mods no próprio Windhawk e clique em "Salvar desta
máquina" na aba Windhawk (`envsetup windhawk-export`). "Aplicar backup"
(`envsetup windhawk-import`, ou a etapa Aparência do perfil) instala os mods que faltam e grava
as configurações. A dock da barra de tarefas é a do mod "Ilha e Dock estilo Mac": liga e desliga
no cartão da aba Preview (`envsetup taskbar-dock`, `--restore`).

## `dotfiles/`

Os arquivos de configuração, organizados por programa. Um arquivo só é implantado se um perfil o
cita em `files`. Para trazer uma config que já existe na máquina: aba Dotfiles › "Adicionar arquivos…" ou "Adicionar pasta…"
(`envsetup add ~/.config/algo`), que copia para `dotfiles/`, registra no perfil e troca o original
por um symlink.

Cuidados:

- **Segredos.** Não guarde chaves e senhas aqui. `dotfiles/zed/settings.json` tem
  `<FIGMA_API_KEY>` e as coleções do Bruno têm `<PASSWORD>` no lugar dos valores. Como o destino é
  um symlink, preencher o valor real no arquivo da home grava o valor na pasta de dados.
- **Específicos desta máquina:** `dotfiles/niri/monitors.kdl` (monitores),
  `dotfiles/bruno/workspace.yml` e caminhos `/home/erik` em `fish/config.fish` e
  `noctalia/config.toml`. Ajuste-os numa máquina com outro usuário.
- `dotfiles/gtk/WhiteSur-Dark-blue` é o tema GTK que o perfil do Ubuntu copia para
  `/usr/share/themes`.
