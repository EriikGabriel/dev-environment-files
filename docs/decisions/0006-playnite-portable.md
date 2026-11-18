## Contexto

O Playnite foi configurado à mão no modo portable, seguindo um vídeo: temas, dezenas de
extensões, capas, trailers e músicas de tema baixados. Refazer isso numa máquina nova leva horas.
No modo portable tudo fica em `C:\Playnite`, mas a pasta tem ~21 GB (18 GB só em
`ExtraMetadata`). Os trailers não tocavam depois da instalação automática: o arquivo de resposta
removia o Windows Media Player, e o player do WPF que o Playnite usa depende do `wmp.dll`.

## Decisão

- **Snapshot fora do repo.** A pasta não vai para o git. `config/playnite.yaml` guarda `path` (a
  pasta), `store` (onde fica a cópia), `exclude` e `start_menu`. A cópia fica em
  `<store>\Playnite` e um manifesto (`envsetup-playnite.json`: data, máquina, pasta de origem,
  tamanho, temas, extensões) fica ao lado, fora da árvore espelhada.
- **Quase nada fica de fora.** Só `JITProfiles` (gerado pelo .NET) e os logs. `cache` e
  `browsercache` (~250 MB) vão junto: o segundo tem os cookies de login das integrações de loja, e
  deixá-lo de fora pediria login de novo.
- **robocopy.** Salvar é `/MIR` (só copia o que mudou e apaga do snapshot o que saiu da pasta);
  restaurar é `/E` por padrão, para não apagar nada de uma pasta existente, e `/MIR` com
  `--mirror`. Pastas excluídas são passadas com caminho completo, para `cache` não pegar as
  subpastas `Cache` das extensões. Códigos de saída abaixo de 8 são sucesso.
- **Playnite fechado.** Salvar e restaurar recusam rodar com `Playnite.DesktopApp` ou
  `Playnite.FullscreenApp` aberto: o banco LiteDB não é copiado de forma consistente com ele aberto.
- **Caminhos.** Extensões guardam caminhos completos (ffmpeg, yt-dlp, imagens). Se a pasta de
  destino não é a de origem, os `.json` da raiz e de `ExtensionsData` têm a pasta trocada, só
  como segmento inteiro (`C:\Playnite\` e não `C:\PlayniteOld`).
- **Requisitos fora da pasta** são verificados e corrigidos à parte (`playnite-fix`): Windows
  Media Player (`Add-WindowsCapability`, elevado), atalho no menu Iniciar
  (`register-portable-app.ps1`) e caminhos de extensões fora da pasta; o `ffmpeg-normalize` é
  instalado com o pip e a extensão passa a apontar para onde ele ficou.
- **Windows Media Player.** O `autounattend.xml` deixou de remover `Media.WindowsMediaPlayer` (no
  XML e no link do gerador, para que regenerar mantenha a escolha); o resumo do arquivo avisa se
  um arquivo novo voltar a removê-lo. O item `windows-media-player` reinstala o recurso em
  máquinas instaladas antes.
- **Método `playnite` no catálogo.** O item `playnite` conta como instalado quando a pasta tem
  `Playnite.DesktopApp.exe`; instalar é restaurar o snapshot e corrigir o Windows Media Player.

## Consequências

- O snapshot depende de um disco ou pasta que exista na máquina nova; sem ele o item falha com
  uma mensagem clara.
- A pasta de dados de uma extensão é achada pelo GUID no id ou, quando não está nele, pelo GUID
  escrito em UTF-16 na DLL. Extensões em script (PowerShell/IronPython) podem ficar sem pasta.
- O `AutoBackupDir` do Playnite aponta para o AppData do usuário e não é ajustado.
- O fluxo foi testado com robocopy real numa pasta falsa (salvar, restaurar em outra pasta,
  reescrita de caminhos). A leitura de temas, extensões, tamanhos e requisitos rodou contra o
  `C:\Playnite` real; salvar o snapshot real (21 GB) e a correção elevada do Windows Media Player
  não rodaram.
