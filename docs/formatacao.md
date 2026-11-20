# Formatar o PC: o que volta, o que não volta e por quê

Este guia diz exatamente o que o envsetup refaz numa instalação limpa do Windows com o perfil
**Gaming · Windows**, o que fica por sua conta e o que acontece em cada situação: com o HD, sem o
HD, sem o GitHub atualizado, sem internet. O passo a passo do dia a dia está na aba **Backup** do
app; aqui está o porquê de cada passo.

Termos usados:

- **C:**: o SSD com o Windows. É o que se formata.
- **HD**: o disco D:, que **não** se formata. Tem duas pastas do envsetup:
  - `D:\envsetup`: o *store*, com a cópia do Playnite, a BIOS do RetroArch e as senhas.
  - `D:\Games`: os jogos. Os do RetroArch ficam em `D:\Games\Retro\<sistema>` e os das lojas em
    `D:\Games\<loja>`.
- **GitHub**: este repositório. Guarda as configurações, os saves e os arquivos `config/`.
- **Pasta de dados**: a pasta que o app usa. Pode ser um clone do repositório ou a cópia que o
  instalador do app faz em `Documentos\envsetup`.

---

## Desenvolvimento no CachyOS

Para reinstalar o ambiente de desenvolvimento, salve ou envie este repositório antes de formatar.
Depois da instalação limpa:

1. Instale o envsetup com `curl -fsSL https://github.com/EriikGabriel/dev-environment-files/releases/latest/download/install.sh | bash`.
2. Escolha os dados incluídos ou o clone atualizado do repositório.
3. Use o perfil **Dev · CachyOS (niri + Noctalia)** e aplique as etapas marcadas.
4. Reinicie a sessão do Niri para carregar Noctalia, Vicinae, o tema de ícones e os atalhos.

O perfil reinstala os apps essenciais, runtimes, Docker/PostgreSQL, Fish + Starship, Kitty, Zed,
Vicinae, Niri + Noctalia, plugins, ícones MacTahoe, wallpaper, fontes e as configurações de
workspace. Identidade Git, chave SSH e autenticação do GitHub são reaplicadas quando preenchidas;
as contas dos aplicativos e credenciais locais precisam ser conectadas novamente.

O que não é guardado: bancos de dados e volumes do Docker/PostgreSQL, projetos fora de `~/www`,
segredos, sessões de aplicativos, configurações geradas pelo Noctalia que não estejam em
`config.toml`/`plugins.json` e arquivos específicos de hardware em `monitors.kdl`.

## Resumo

| Situação | O que acontece |
|---|---|
| Formatou só o C:, salvou e enviou tudo antes | Tudo da tabela "O que volta" abaixo volta. Você refaz só a lista "O que fica por sua conta". |
| Sem o HD (perdido, formatado ou desconectado) | Os programas, drivers, lojas, Windhawk, o RetroArch (com configurações e saves) e o Playnite (com temas, extensões, configurações, emuladores e pastas de ROM) voltam. Os jogos, a BIOS, as senhas e, no Playnite, o tempo jogado, os favoritos e os trailers se perdem. |
| HD presente, mas o GitHub ficou para trás (esqueceu de enviar) | Volta tudo, mas as configurações e os saves voltam **na versão do último envio**. |
| Usou só os dados do instalador, sem clonar | Igual ao anterior, com a versão da época em que o instalador foi gerado. |
| Sem internet | Quase nada: só as cópias que vêm do HD funcionam. |

---

## O que volta, etapa por etapa

O perfil de jogos tem as etapas abaixo, na ordem em que rodam. **Marcado** é o que já vem
selecionado na aba Configurar; dá para desmarcar qualquer item. Uma etapa que falha fica
registrada e as seguintes **continuam**.

| Etapa | Itens (marcados / opcionais) | De onde vem | Precisa do HD? |
|---|---|---|---|
| **Essenciais** | 7-Zip, Visual C++, DirectX, Windows Media Player, Raycast e as extensões dele (`windows/raycast/extensions.txt`) / Scoop, PowerToys, Ninite | winget (o Raycast vem da Microsoft Store; cada extensão abre na loja do Raycast para um clique em Install) | não |
| **Drivers e hardware** | o software da sua placa de vídeo (NVIDIA App ou AMD Adrenalin, escolhido pela placa detectada) / MSI Afterburner, HWiNFO | site da NVIDIA ou da AMD, winget | não |
| **Periféricos** | MCHOSE HUB, Attack Shark Driver, SignalRGB, 8BitDo Ultimate Software V2 | site do fabricante, winget | não |
| **Lojas e launchers** | Steam, Epic Games, Hydra Launcher / GOG Galaxy, EA app, Ubisoft Connect, Battle.net | winget | não |
| **Aparência** | Windhawk e os mods de `windows/windhawk/backup.json`, com as configurações (dock, menu Iniciar, Explorer…), e as opções da ilha do "Ilha e Dock estilo Mac" (`windows/windhawk/mac-island-dock.txt`) | o instalador do Windhawk 2 no GitHub (a versão mais recente, pré-release inclusa) e o repositório de mods dele | não |
| **Playnite** | o Playnite portable em `C:\Playnite`: biblioteca, capas, trailers, temas, extensões e configurações | HD (`D:\envsetup\Playnite`) e GitHub (`windows/playnite`); sem o HD, o programa e os add-ons vêm da internet | só a biblioteca de jogos e as mídias |
| **Emuladores** | o RetroArch portable em `C:\Emuladores\RetroArch`: programa, 7 cores, configurações, saves, estados, playlists e BIOS | buildbot.libretro.com, GitHub (`windows/retroarch`) e HD (BIOS) | só a BIOS e as senhas |
| **Comunicação e navegador** | Discord, Opera GX / Brave, Spotify, OBS Studio | winget | não |
| **Otimizações** | compressão de memória desligada (`Disable-MMAgent -MemoryCompression`; pede elevação e vale depois de reiniciar) / políticas de grupo (`config/policies.yaml`) | o próprio Windows | não |
| **Apps da Steam** | Wallpaper Engine | a Steam (você precisa estar logado) | não |
| **Barra de tarefas** | a barra (a dock do Windhawk): Explorador, Spotify, Discord, o seu navegador, Playnite, RetroArch e Hydra Launcher; escondida automaticamente; menu Iniciar e área de trabalho limpos (`config/layout.yaml`) | o próprio Windows | não |

### Detalhes do que cada etapa restaura

- **Playnite.**
  - A cópia do HD traz o programa, a biblioteca, as capas, os trailers, os temas e as extensões.
  - Por cima, entram as configurações do GitHub (`config.json`, `fullscreenConfig.json` e as de
    cada extensão), com as chaves de API devolvidas do HD.
  - Se o RetroArch ou os jogos mudaram de lugar desde o salvamento, os caminhos na biblioteca são
    corrigidos.
  - No fim, entra no menu Iniciar e o Windows Media Player é instalado (ele toca os trailers).
  - Os caches (`cache`, `browsercache`) não são guardados: o Playnite refaz sozinho.
  - **Sem o HD**, o Playnite é baixado (versão de `config/playnite.yaml`), os temas e extensões
    de `windows/playnite/addons.yaml` vêm do banco de add-ons do Playnite e o GitHub traz as
    configurações e a estrutura da biblioteca. O que só o HD guarda está em
    [configuracao.md](configuracao.md#o-que-o-github-garante-do-playnite-e-o-que-só-o-hd-guarda).
- **RetroArch.**
  - Baixa o RetroArch na versão gravada em `config/retroarch.yaml` (hoje a 1.22.2), os pacotes
    de assets e os 7 cores.
  - Copia a BIOS (pasta `system`) do HD.
  - Aplica as configurações, os saves, os estados, as capas e as playlists do GitHub, e devolve
    a senha do RetroAchievements.
  - Aponta as playlists para os jogos em `D:\Games\Retro` e cria o atalho no menu Iniciar.
  - Leva uns 3 minutos e ~500 MB de download.
- **Jogos.** Não são copiados: já moram em `D:\Games`. O RetroArch e o Playnite apontam para lá.

### Antes do perfil: o pendrive (opcional)

Reinstalando pelo pendrive gravado na aba Instalação, o `windows/autounattend.xml` instala o
Windows sem perguntas e já faz isto:
- idioma pt-BR e a conta configurada no arquivo;
- remove os apps que vêm de fábrica (Copilot, OneDrive, Teams…) e desliga widgets e sugestões;
- liga o plano de energia Desempenho Máximo;
- tira a Galeria do painel do Explorador de Arquivos;
- deixa o Windows e os apps no modo escuro (vale para toda conta criada);
- roda o Ninite no primeiro login.

O mesmo arquivo pode ser aberto no gerador do schneegans.de pela aba Instalação.

---

## O que fica por sua conta

Isto **não** é guardado nem refeito pelo envsetup. Confira antes de formatar.

**Arquivos e saves fora do envsetup**
- **Suas pastas do usuário no C:** (Documentos, Área de Trabalho, Downloads, Imagens, Vídeos).
  Copie para o HD o que quiser manter.
- **Saves de jogos de PC** que não usam nuvem. Muitos ficam em `Documentos\My Games`,
  `%APPDATA%` ou `%LOCALAPPDATA%`. A Steam Cloud, a Epic e a maioria das lojas guardam na
  nuvem; os outros, copie à mão.
- **Dados do Hydra Launcher**: a biblioteca e as configurações ficam em `%APPDATA%` e não são
  salvas. Os jogos baixados por ele ficam onde você mandou (use `D:\Games\Hydra`).

**Contas e logins**
- Steam, Epic, Hydra, Discord, Raycast, Spotify, NVIDIA App e as lojas opcionais: entre de novo.
- **Opera GX**: favoritos, senhas e abas voltam pela sincronização da conta Opera, se ela estiver
  ligada.

**Configurações de programas que o envsetup não salva**
- **NVIDIA App** e **AMD Adrenalin** (filtros, perfis de jogo, overlay).
- **SignalRGB** (efeitos e layouts).
- **MCHOSE HUB** (equalização do headset).
- **Raycast**: os atalhos e as configurações das extensões (a conta Raycast pode sincronizar parte).
  As extensões em si voltam pela lista `windows/raycast/extensions.txt`, com um clique em
  Install em cada uma.
- **Discord** e **OBS** (cenas, perfis).
- **PowerToys**.
- A configuração do **mouse Attack Shark** e do **controle 8BitDo** fica gravada nos próprios
  aparelhos: os programas só são reinstalados.

**O que a Steam e as outras lojas precisam de você**
- **Reconhecer os jogos do HD.** Na Steam: Configurações > Armazenamento > Adicionar unidade >
  `D:\Games\Steam`; os jogos que já estão lá aparecem como instalados. Na Epic: instale o jogo
  apontando para a pasta existente e ela verifica os arquivos em vez de baixar tudo.
- **Wallpaper Engine**: a Steam abre a janela de instalação, e os papéis de parede da Oficina são
  baixados de novo.

**Partes do Windows que não estão no perfil**
- **Ícones** (`config/icons.yaml`) não são uma etapa do perfil: aplique pela aba **Preview**
  depois que os programas estiverem instalados. A barra de tarefas é a última etapa do perfil.
- **Políticas de grupo**: na etapa Otimizações, só a compressão de memória vem marcada; as
  políticas vêm **desmarcadas**. Marque, ou aplique pela aba Políticas.
- **Reiniciar** depois de aplicar o perfil: a compressão de memória só desliga após o reinício.
- **Outros drivers**: chipset AMD, áudio da placa-mãe, Wi-Fi e Bluetooth. O Windows Update
  instala a maior parte; o chipset AMD vale baixar do site da AMD.
- **Ativação do Windows**: volta sozinha pela licença digital ligada à placa-mãe e à sua conta
  Microsoft.
- **Outras configurações do Windows**: tema, papel de parede, som, mouse, teclado e o que não
  está nas políticas.

---

## Cenários

### 1. Formatação normal: só o C:, com o HD e o GitHub em dia

É o caminho para o qual tudo foi feito.

1. **Antes de formatar**, na aba Backup: escolher o HD, fechar o Playnite e o RetroArch, salvar
   os dois e enviar para o GitHub. O último passo também grava no HD de qual repositório e branch
   veio o backup (`D:\envsetup\envsetup-git.json`).
2. Formate **só a partição do C:**. No instalador do Windows, não apague nem formate o D:.
3. **Depois de formatar**:
   - Instale o app e escolha "Usar dados incluídos".
   - Na aba Backup, clique em **Baixar do GitHub**: instala o Git, clona o branch do último
     backup e passa a usá-lo.
   - Aplique o perfil de jogos.
4. Faça a lista de "O que fica por sua conta".

**Tempo:** os programas dependem da internet. O Playnite copia ~26 GB do HD (alguns minutos) e o
RetroArch leva uns 3 minutos.

### 2. Sem os dados do HD

Vale para o HD perdido, formatado, desconectado, ou se você **não quiser usar** o que está nele.
Tudo o que vem do GitHub e da internet continua funcionando.

| Item | Sem o HD |
|---|---|
| Programas, drivers, periféricos, lojas, Hydra, Windhawk, Discord, Raycast… | voltam normalmente |
| **Playnite** | volta da internet e do GitHub: o programa, os temas e extensões (menos os que não estão no banco de add-ons, hoje o PS5reborn), as configurações, os emuladores e as pastas de ROM. Os jogos são importados de novo pelas lojas e scanners, **sem** tempo jogado, favoritos e notas; capas e trailers são baixados de novo. |
| **RetroArch** | volta: programa e cores baixados, configurações, saves, estados e playlists do GitHub |
| BIOS do RetroArch | **perdida**: jogos de PlayStation (Beetle PSX HW) não abrem até você pôr a BIOS na pasta `system` |
| Senha do RetroAchievements e chaves de API do Playnite | **perdidas**: entre de novo nos programas |
| **Jogos** (`D:\Games`) | **perdidos**: as playlists do RetroArch mostram os jogos, mas eles não abrem. Os das lojas são baixados de novo. |

**Para usar o HD, mas não os dados guardados nele** (começar o RetroArch do zero com as
configurações do GitHub):
- Esvazie o `store` antes de aplicar, com `envsetup set-retroarch --store ""` e
  `envsetup set-playnite --store ""`, ou escolha outra pasta na aba Backup.
- O RetroArch e o Playnite voltam só com o que está no GitHub e na internet.

**Para não restaurar nada do Playnite e do RetroArch:** desmarque as etapas **Playnite** e
**Emuladores** na aba Configurar. O resto do perfil não depende delas.

### 3. O GitHub ficou para trás

Se você salvou no HD mas **não enviou** para o GitHub (ou enviou de outro branch), o clone traz
as configurações e os saves do último envio. O perigo:

- **O RetroArch** aplica os saves e estados do GitHub por cima. O que você jogou depois do último
  envio se perde.
- **O Playnite** copia o HD, que está atualizado, mas depois aplica as configurações do GitHub,
  que são mais antigas. As configurações do Playnite e das extensões voltam à versão do último
  envio; a biblioteca e as mídias, que vêm do HD, ficam atualizadas.

Por isso o passo **Enviar para o GitHub** existe e a aba Backup só diz "Pronto para formatar"
quando não há nada pendente. Se esqueceu, envie da máquina velha antes de formatar. Se já
formatou, o que estava só no C: não tem como voltar.

### 4. Só os dados do instalador, sem clonar

O instalador do app leva uma cópia de `config/` e `windows/` **da época em que foi gerado**.
Aplicar o perfil com ela é igual ao cenário 3: configurações e saves daquela época. Use sempre
**Baixar do GitHub** na aba Backup. Se o Git não puder ser instalado, clone à mão com
`git clone -b <branch> https://github.com/EriikGabriel/dev-environment-files` e escolha a pasta
pelo botão da pasta de dados.

### 5. Sem internet

O winget, os sites dos fabricantes, o buildbot do RetroArch, os mods do Windhawk e o GitHub
precisam de internet.
- **O que funciona offline:** a cópia do Playnite que vem do HD. O RetroArch também, mas só com
  "Guardar também o programa inteiro no HD" ligado na aba RetroArch (`copy_program`).
- **O resto:** espera a internet. Aplique o perfil de novo depois: o que já estiver instalado é
  pulado.

### 6. O HD mudou de letra

Se `D:\envsetup` não existe, mas a mesma pasta existe em outra letra com o manifesto dentro
(`E:\envsetup`), o envsetup usa essa. Para os jogos, clique em **Trocar** a pasta de jogos, na aba
RetroArch, e escolha a pasta na letra nova. Como a pasta antiga não existe mais, nada é movido:
as playlists e a biblioteca do Playnite passam a apontar para a letra nova
(`envsetup retroarch-move-games E:\Games\Retro` faz o mesmo).

### 7. Trocar o HD por outro lugar (pendrive, NAS, nuvem)

- **O store:** em **Escolher o HD**, na aba Backup, aponte para a nova pasta e salve o Playnite e
  o RetroArch de novo. A primeira vez copia tudo.
- **Os jogos**, se forem para o mesmo lugar:
  - ligue "Copiar também os jogos" na aba RetroArch, para copiar a cada salvamento; ou
  - use **Trocar** a pasta de jogos e responda "Mover", para que passem a morar lá.
- **Pasta sincronizada na nuvem:** espere sincronizar antes de formatar e antes de restaurar.
  O Playnite sozinho tem ~26 GB.

### 8. Formatar também o HD

Copie `D:\envsetup` e `D:\Games` para outro disco antes e use o cenário 7 para apontar para lá.
Sem isso, é o cenário 2.

---

## Para outra pessoa: instalar com as configurações do dono

O app foi feito para o dono do repositório, mas qualquer pessoa pode usá-lo para formatar o PC
com as mesmas escolhas, ligando e desligando o que quiser.

**Instalar o app.** Pela página de releases do GitHub:

- **Windows** (PowerShell):
  `irm https://github.com/EriikGabriel/dev-environment-files/releases/latest/download/install.ps1 | iex`,
  ou baixe o `envsetup_*_x64-setup.exe`.
- **Ubuntu / Debian e Arch** (terminal):
  `curl -fsSL https://github.com/EriikGabriel/dev-environment-files/releases/latest/download/install.sh | bash`.
  Instala o `.deb` no Ubuntu e o `.AppImage` no Arch (com o `fuse2`).

Na primeira execução, escolha **Usar dados incluídos**: o app copia as configurações que vieram
no instalador para `Documentos\envsetup`.

**De quem é a configuração.** A tela Início pergunta "De quem é esta configuração?". Quem instalou
pelo instalador já começa em **De outra pessoa**; o dono, que usa um clone do Git, em **É minha**.
A escolha fica em `local.yaml`, que nunca vai para o git.

| | Dono | Outra pessoa |
|---|---|---|
| Programas, drivers, lojas, Windhawk, políticas, barra de tarefas | tudo, já marcado | tudo, já marcado; desmarque o que não quiser |
| Driver de vídeo | o da placa detectada | o da placa detectada |
| Programas dos periféricos (MCHOSE, Attack Shark, 8BitDo) | marcados se o aparelho estiver conectado | idem: sem o aparelho, vêm desmarcados |
| Playnite: temas, extensões, configurações, emuladores, pastas de ROM | sim | sim, **sem** as contas do dono (Steam ID, IDs de usuário) |
| Playnite: biblioteca de jogos, tempo jogado, mídias | do HD do dono | não (cada um tem a sua; os jogos vêm das suas lojas) |
| RetroArch: programa, cores, configurações, opções dos cores, capas | sim | sim, **sem** o usuário do RetroAchievements e o apelido do netplay |
| RetroArch: saves, estados, playlists e histórico | sim | **não** |
| Aba Backup: enviar e baixar do GitHub | sim | não (o backup de quem não é o dono é o próprio HD) |

**Ao salvar a sua própria configuração** (Salvar nas abas Playnite e RetroArch), a pasta de dados
passa a guardar o que é seu e o app passa a tratar você como dono. Assim, uma restauração futura
não deixa de fora os seus saves.

**Antes de instalar o Windows pelo pendrive:** o `autounattend.xml` cria a conta com o nome do
dono. A aba Instalação mostra o nome; para trocar, use **Customizar no site**, mude a conta,
baixe o XML e use **Importar XML…**.

**O que cada um ajusta.** As pastas (`D:\Games`, `C:\Emuladores`), o HD, as etapas e os itens se
mudam nas abas, sem editar arquivos. O que o dono deixa como padrão está em `config/`; veja
[configuracao.md](configuracao.md).

---

## Por que cada coisa mora onde mora

- **GitHub**: só o que é pequeno, muda à mão e pode ser público.
  - Pode ir: configurações, saves e playlists, com as senhas, chaves de API e tokens apagados
    antes do envio.
  - Não pode ir: jogos e BIOS, que têm direitos autorais e passam do limite de 100 MB por arquivo
    do GitHub; e o Playnite, que tem dezenas de GB.
- **Internet**: o que é público e baixa rápido (programas, drivers, RetroArch, cores). Não ocupa
  o HD e vem sempre na versão gravada.
- **HD**: o que não pode ir para o GitHub nem ser baixado de novo com facilidade. São os jogos, a
  BIOS, a biblioteca e os trailers do Playnite (lentos de baixar) e as senhas.

Detalhes técnicos de cada arquivo estão em [configuracao.md](configuracao.md#onde-fica-cada-parte-dos-programas-portáteis).
