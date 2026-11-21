## Contexto

O perfil de jogos só começava depois de o Windows estar instalado. A instalação em si era feita
com um pendrive Ventoy, um `autounattend.xml` do gerador schneegans.de e um instalador do
Ninite, guardados só no pendrive. Depois dela, um conjunto de políticas de grupo era ajustado à
mão no gpedit, seguindo um vídeo.

## Decisão

- **O arquivo de resposta mora no repo** (`windows/autounattend.xml`, com o `ventoy.json`), e o
  perfil aponta para eles em `media:`. O envsetup não edita o XML: o gerador é a ferramenta para
  isso. Ele só lê o link que o gerador deixa num comentário, para resumir o que o arquivo faz e
  para reabrir o site com as mesmas opções; uma versão nova entra por `set-unattend`.
- **O link é aberto por uma página local.** Ele tem cerca de 9 mil caracteres, mais do que o
  shell do Windows aceita numa URL (2083), então `open-url` grava um HTML temporário que
  redireciona. `open-url` só abre alvos nomeados (`unattend`, `ninite`...), nunca uma URL vinda
  do front-end.
- **Ninite é um método do catálogo.** `names` é a lista de programas; o instalador é baixado de
  `https://ninite.com/<lista>/ninite.exe` e iniciado por `Start-Process`, que é o que mostra o
  UAC. O Ninite não diz o que está instalado, então o item conta como feito quando já rodou para
  aquela lista exata (marcador no diretório de estado). O `.exe` não vai para o repo.
- **`write-media` grava o pendrive.** O XML vai para o `template` do `ventoy.json`; o
  `ventoy.json` é mesclado chave a chave com o do pendrive; o Ninite vai para a raiz, onde o
  script de primeiro login do XML o procura. Caminhos que sairiam do pendrive são recusados.
- **Políticas pelo registro.** `config/policies.yaml` tem, para cada política, a pasta no gpedit,
  o estado do vídeo, uma explicação, ressalvas e os valores DWORD em `HKLM\SOFTWARE\Policies`
  (ou `...\CurrentVersion\Policies`). A escolha (`video`, `off` ou uma opção) fica no próprio
  arquivo, porque é configuração do ambiente. Aplicar é um script PowerShell elevado; o estado é
  lido sem elevação. Chaves fora dessas duas árvores são recusadas ao carregar.

## Consequências

- Escrever só no registro não basta: o gpedit.msc lê o `Registry.pol` e mostraria "Não
  configurado", e um `gpupdate` pode apagar valores que não estejam nele. O script elevado
  também edita o `Registry.pol` (formato PReg, sem depender do `LGPO.exe`) e roda `gpupdate`.
- Os valores vieram do PDF de capturas que o autor do vídeo publicou. Três pontos não aparecem
  nele: a opção da política de pré-inicialização do Edge (assumido "impedir") e o estado das
  políticas de pesquisa na Web cobertas por um diálogo, que ficaram de fora.
- Os nomes dos valores do registro foram escritos a partir dos modelos ADMX; a leitura do estado
  rodou numa máquina Windows 11, a gravação (elevada) ainda não.
- O estado lê o registro **e** o `Registry.pol`: lendo só o registro, um `Registry.pol` antigo
  ou ilegível passava por aplicado e nunca era regravado. Os valores foram conferidos contra os
  ADMX de um Windows 11 pt-BR: o estado do gpedit sai do modelo, não do nome da política (há
  modelos que gravam 0 para Habilitado, e Desabilitados que são `**del.<nome>`).
- Conduzir o gpedit por UI Automation para gravar cada política foi considerado e descartado:
  são 42 janelas por aplicação, a navegação depende da árvore do MMC ser visível para a
  automação (em algumas máquinas não é, e o "Abrir no gpedit" cai no modo de teclado) e uma
  falha no meio deixa metade aplicada. Gravar o mesmo que o gpedit grava, conferido contra os
  ADMX, dá o mesmo resultado sem depender da janela.
- O catálogo de programas do Ninite é uma cópia da página inicial do site; um nome que não
  estiver nele ainda pode ser digitado, mas o site ignora nomes que não conhece.
- No Windows, `go test ./...` falha em cinco testes antigos que criam symlinks ou esperam
  caminhos do Linux; os testes novos passam no Windows (não foram rodados no Linux).
