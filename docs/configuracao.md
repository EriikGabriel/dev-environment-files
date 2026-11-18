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
config/playnite.yaml       onde fica o Playnite portable e o snapshot dele
windows/autounattend.xml   instalação automática do Windows (com windows/ventoy.json)
windows/windhawk/backup.json   mods do Windhawk e as configurações deles
windows/icons/mactahoe/    pack de ícones (PNG por nome de app)
dotfiles/                  os arquivos de configuração que vão para a home
```

**Três jeitos de mudar uma config.** Pela aba do app que cuida dela, por um comando
(`envsetup set-…`) ou editando o arquivo. Os arquivos da `config/` começam com um comentário que
explica cada campo, e esse comentário é a referência; este guia é o mapa. O app e os comandos
`set-…` reescrevem só o que mudou, então comentários seus nos arquivos editados à mão continuam
lá, exceto em `layout.yaml`, `icons.yaml` e `playnite.yaml`, que o envsetup regrava inteiros (com o
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

## `config/playnite.yaml`: Playnite portable (Windows)

| Campo | O que faz |
|---|---|
| `path` | a pasta do Playnite (padrão `C:\Playnite`) |
| `store` | onde fica o snapshot: a cópia vai para `<store>\Playnite` e um manifesto para `<store>\envsetup-playnite.json` |
| `exclude` | o que fica de fora: nomes com `*` ou ponto são padrões de arquivo; o resto, pastas relativas a `path` |
| `start_menu` | depois de restaurar, põe o Playnite no menu Iniciar e em Configurações › Apps |

A pasta tem dezenas de GB (capas, trailers, músicas), por isso o snapshot fica fora da pasta de
dados, num disco ou pasta sincronizada. Escolha o `store` e salve pela aba Playnite
(`envsetup playnite-save`); numa máquina nova, a etapa Playnite do perfil (ou
`playnite-restore`) copia de volta e corrige os caminhos das extensões se a pasta mudou. A aba
também mostra o que a pasta não leva (Windows Media Player para os trailers, programas
externos) e corrige.

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
