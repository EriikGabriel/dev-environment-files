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

> No Linux os dotfiles são implantados como symlink para a pasta de dados (no Windows, como
> cópia). Se você mover a pasta, aponte o app para o novo lugar e aplique de novo.

### Desinstalar o app depois de configurar

Dá para usar o app só para formatar e depois se livrar dele. Os programas, as políticas, o
RetroArch, o Playnite e os jogos não dependem da pasta de dados. A exceção são os symlinks do
Linux, que ficam quebrados se a pasta sumir. Antes de apagar:

1. Num clone, faça commit e push (aba Backup → "Enviar"), para não perder o que mudou.
2. Na aba **Dotfiles**, clique em **"Desvincular da pasta de dados"** (ou rode
   `envsetup detach`). Cada symlink, de todos os perfis, vira uma cópia do arquivo.
3. Desinstale o app e apague a pasta de dados.

Depois do passo 2, editar uma config da home não muda mais o repositório. Se você aplicar o perfil
de novo, os symlinks voltam (o que estiver no lugar vira `.bak`).

## O app

A versão que está rodando aparece ao lado do nome, na barra lateral; clicar nela (ou "Sobre o
envsetup" na busca, Ctrl K) mostra a versão, o sistema e a pasta de dados. A release de mesmo
nome no GitHub é a que tem o instalador dessa versão.

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
| **RetroArch** (Windows) | cores, sistemas e jogos, playlists, o snapshot do RetroArch e dos jogos e o que precisa de correção |

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
envsetup retroarch --sizes | set-retroarch --store=D:\envsetup [--copy-program] | retroarch-save | retroarch-restore
envsetup retroarch-fix playlists                # aponta as playlists para os jogos de novo
envsetup retroarch-open app                     # abre o RetroArch (ou folder, games, store)
envsetup retroarch-move-games D:/Games/Retro     # move os jogos e corrige playlists e Playnite
envsetup playnite-close | playnite-relocate --from=C:/old --to=D:/new
envsetup backup [--fetch] | backup-push | backup-pull   # o que falta antes de formatar; enviar; atualizar
envsetup backup-clone --dir=... --url=... --branch=...  # máquina nova: instala o Git e clona
```

## Perfil de jogos: drivers e periféricos

- **Essenciais.** Runtimes dos jogos (Visual C++, DirectX), o 7-Zip, o Windows Media Player e o
  Raycast. O Raycast vem da Microsoft Store (ID `9PFXXSHC64H3` no winget), porque não está no
  repositório comum do winget.
- **Extensões do Raycast.** A sincronização do Raycast não traz as extensões da loja para outro
  Windows, e elas só se instalam com um clique. A lista fica em `windows/raycast/extensions.txt`
  (`autor/nome`); o item `raycast-extensions` abre no Raycast a página de cada uma que falta e
  espera você clicar em Install. Atualize a lista com `windows\raycast-extensions.ps1 -Save`.
- **Deno.** No Windows, o app instala o Deno sozinho, em segundo plano, quando abre e não o
  encontra (`winget install DenoLand.Deno`, só para o usuário, sem perguntas). Uma falha (sem
  internet, sem winget) só vai para o console, e o app tenta de novo na próxima vez
  (`envsetup ensure-deno`).
- **Lojas e launchers.** Steam, Epic Games e Hydra Launcher marcados; GOG, EA, Ubisoft e Battle.net
  opcionais. Configure a pasta de instalação de cada um para `D:\Games\<loja>`.
- **Placa de vídeo.** A etapa Drivers traz o NVIDIA App e o AMD Software (Adrenalin). Vem
  marcado só o da placa desta máquina, e o assistente mostra o motivo ("placa NVIDIA detectada").
- **Periféricos.** A etapa Periféricos instala, já marcados, o MCHOSE HUB (headset V9 Pro), o
  Attack Shark Driver (mouse X3 Pro), o SignalRGB e o 8BitDo Ultimate Software V2 (controle
  Ultimate 2.4G). O MCHOSE, o Attack Shark e o 8BitDo vêm marcados só se o aparelho estiver conectado (o assistente
  mostra "conectado: MCHOSE V9 PRO"; campo `devices` em `config/apps.yaml`); o SignalRGB vem sempre.
  Desmarque o que não quiser. Eles não têm configuração para restaurar: as do
  mouse e do controle ficam gravadas nos próprios aparelhos.

## Formatar o PC

**O que volta e o que não volta** numa instalação limpa, e o que acontece sem o HD, sem o GitHub
atualizado ou sem internet: **[formatacao.md](formatacao.md)**.

**Use a aba Backup.** Ela é um passo a passo com duas telas, "Antes de formatar" e "Depois de
formatar". Cada passo mostra se está pronto, o que falta e um botão para resolver, e o próximo
passo pendente fica destacado. Numa máquina sem Playnite nem RetroArch, ela já abre em "Depois".

Só o **C:** (o SSD) é formatado. Cada coisa mora num destes lugares:

| Onde | O quê |
|---|---|
| **GitHub** | configurações e saves do Playnite e do RetroArch, sem senhas (`windows/playnite`, `windows/retroarch`) e os `config/*.yaml` |
| **Internet** | o RetroArch e os cores, os drivers e os programas do perfil |
| **HD** (`D:\envsetup`) | o Playnite (biblioteca, capas, trailers, temas, extensões), a BIOS e as senhas tiradas do git |
| **HD** (`D:\Games`) | os jogos: os do RetroArch em `D:\Games\Retro\<sistema>`, os das lojas em `D:\Games\<loja>` |

Os jogos não são copiados. Eles já moram no HD, e o RetroArch e o Playnite apontam para lá.
Instale todo jogo novo em `D:\Games`: na Steam, em Configurações > Armazenamento > Adicionar
unidade; na Epic, escolhendo a pasta na instalação. Depois de formatar, adicione `D:\Games\Steam`
de novo na Steam, e ela reconhece os jogos que já estão lá.

**Antes de formatar** (aba Backup)

1. **Escolher o HD:** uma pasta para o Playnite e o RetroArch num disco que não será formatado.
2. **Fechar o Playnite e o RetroArch:** o botão fecha o Playnite pelo `--shutdown` dele, que
   grava a biblioteca antes de sair.
3. **Salvar o Playnite:** a primeira vez copia ~26 GB; depois, só o que mudou.
4. **Salvar o RetroArch.**
5. **Enviar para o GitHub:** faz commit de `config/` e `windows/` e push. Também grava no HD de
   onde veio o backup (`D:\envsetup\envsetup-git.json`: repositório e branch), para a máquina
   nova saber o que baixar.
6. **Pendrive do Windows (opcional):** na aba Instalação.

**Depois de formatar**

1. Instale o app e escolha "Usar dados incluídos". Na aba Backup, **Baixar do GitHub** instala o
   Git (pelo winget) se faltar, clona o branch do último backup ao lado da pasta de dados e passa
   a usá-lo. Num clone que já existe, o botão é **Atualizar** (`git pull`).
2. **Conectar o HD.** Se a letra mudou, a pasta é encontrada sozinha.
3. **Restaurar tudo:** **Aplicar o perfil** leva à aba Configurar. O perfil de jogos instala
   drivers, periféricos e lojas e restaura o Playnite (etapa Playnite) e o RetroArch (etapa
   Emuladores). Também há botões para restaurar só um dos dois.
4. **Conferir** o que as abas Playnite e RetroArch ainda apontam (menu Iniciar, Windows Media
   Player).

**O Playnite e o RetroArch juntos.** A biblioteca do Playnite guarda o caminho completo do
RetroArch (o emulador) e de cada ROM. Ao salvar, o manifesto do Playnite registra onde estavam o
RetroArch e os jogos. Ao restaurar, se `config/retroarch.yaml` diz outro lugar, a biblioteca é
corrigida (`windows/playnite-relocate.ps1`, com a LiteDB do próprio Playnite). Para trocar a
pasta dos jogos, use **Trocar** na aba RetroArch e responda "Mover". Ou rode
`envsetup retroarch-move-games <pasta>`: copia, confere, apaga a pasta antiga e corrige as
playlists e a biblioteca do Playnite.

**Trocar o HD por outro lugar** (pendrive, NAS, pasta sincronizada): escolha a nova pasta em
"Escolher o HD" e salve os dois de novo. Se os jogos também forem para lá, ligue "Copiar também
os jogos" na aba RetroArch, ou mova a pasta com "Trocar". Formatando também o HD, copie
`D:\envsetup` e `D:\Games` para outro disco antes.

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
  `gpupdate`. Algumas só valem depois de sair da sessão ou reiniciar. O `gpt.ini` ao lado ganha a
  extensão de registro e uma versão nova a cada aplicação, senão o Windows ignora o arquivo.
  - **"Falha ao abrir o Objeto de Política de Grupo… Final do arquivo alcançado".** Era o
    `Registry.pol` gravado pelas versões até a 0.1.7, sem o caractere nulo que fecha cada nome.
    Aplicar as políticas de novo com esta versão lê aquele arquivo e o regrava no formato certo.
  - **Para mexer à mão**, cada política da aba Políticas tem dois botões:
    - **Abrir no gpedit** abre o Editor de Política de Grupo e navega até a política: expande
      Configuração do Computador > Modelos Administrativos > a pasta dela, seleciona e abre a
      janela de edição. O gpedit não aceita caminho na linha de comando, então o app conduz a
      janela dele (`windows/open-policy.ps1`, com UI Automation). Como o gpedit roda como
      administrador, o Windows pede permissão uma vez.
    - **Ver no registro** abre o Editor do Registro direto na chave que a política grava (ou na
      parte dela que já existe, se ainda não foi aplicada). Feche o Editor do Registro antes,
      porque ele só vai para uma chave quando abre.
  - **Se a navegação no gpedit parar** (um nome diferente na sua versão do Windows), a janela
    fica na última pasta encontrada e a mensagem diz o que falta. Se a pasta abrir mas a política
    não for encontrada, o nome dela vai para a área de transferência, para você buscar. Na linha
    de comando: `envsetup open-policy <id> [--registry]`.
  - O gpedit só existe nas edições Pro, Enterprise e Education.
- **Barra de tarefas e menu Iniciar.** A barra é escrita num `TaskbarLayout.xml` apontado pela
  política "Start layout"; o menu Iniciar é limpo por `ConfigureStartPins` e os ícones da área de
  trabalho por `HideIcons`. "Esconder automaticamente" é o byte 8 de `StuckRects3`. O Playnite e
  o RetroArch são portables: o envsetup registra os dois como apps do usuário (atalho no menu
  Iniciar e entrada em Aplicativos instalados), o que permite fixar e achar pela pesquisa. Se o
  programa já está na pasta e só falta o registro, o item faz só isso.
- **Ícones.** O Windows não tem tema de ícones; o envsetup troca o ícone dos atalhos (área de
  trabalho, menu Iniciar, barra) e, se o pack tiver, os de Este Computador, Lixeira, Usuário e
  Rede. Um app com arquivo no pack recebe esse arquivo; os outros recebem o ícone original numa
  placa arredondada. O ícone original de cada atalho fica em
  `%ProgramData%\envsetup\icons\manifest.json` para desfazer. Apps que recriam o atalho ao
  atualizar (Discord, Steam) voltam ao ícone deles até você aplicar de novo. O pack MacTahoe vem
  do [MacTahoe-icon-theme](https://github.com/vinceliuice/MacTahoe-icon-theme) (GPL-3.0);
  `go run ./cmd/iconpack` o refaz.
- **Windhawk.** Precisa do Windhawk 2 (o 1.x não tem o `windhawk-cli`). O item `windhawk` não usa
  o winget, que só tem o 1.x: `windows/install-windhawk.ps1` pega a release mais recente de versão 2
  ou maior no GitHub (hoje uma pré-release, a 2.0 alpha 8; quando sair a final, ela passa a ser
  a escolhida), confere o SHA-256 e a assinatura e instala sem perguntas, com um pedido de
  elevação. Se a versão já estiver instalada, não faz nada; rodar o script à mão também atualiza
  uma versão 2 mais antiga. Aplicar baixa os mods do
  repositório do Windhawk, então precisa de internet. Mods que só existem na máquina ficam como
  estão. A dock é a do mod "Ilha e Dock estilo Mac"; ligar instala o mod se faltar e lembra o modo
  anterior da barra para desligar.
- **Playnite.** Salvar e restaurar recusam rodar com o Playnite aberto. Salvar atualiza
  `windows/playnite` (configurações, sem chaves de API) e o HD (o resto, sem os caches).
  Restaurar não apaga o que já está na pasta, a não ser com `--mirror`. O Windows Media Player, que toca os trailers,
  vem no item `windows-media-player` da etapa Essenciais (o `autounattend.xml` não o remove mais).

- **RetroArch.** Salvar, restaurar e corrigir as playlists recusam rodar com o RetroArch aberto
  (ele grava saves e o `retroarch.cfg` enquanto roda). Salvar atualiza `windows/retroarch`
  (configurações, playlists, saves, estados) e o HD (BIOS e jogos). Restaurar baixa o RetroArch e
  os cores quando faltam. Os jogos nunca vão para o git:
  veja [configuracao.md](configuracao.md#configretroarchyaml-retroarch-portable-e-jogos-windows).

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
