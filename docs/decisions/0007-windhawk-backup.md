## Contexto

Os mods do Windhawk (dock e ilha estilo Mac, menu Iniciar, Explorer, cantos, animações) eram
configurados à mão, mod por mod. Um `taskbar-dock.ps1` escrevia direto no registro as
configurações do Windows 11 Taskbar Styler para desenhar uma dock. O Windhawk 2 passou a
exportar tudo num JSON (`windhawk-user-data-v1`) e traz o `windhawk-cli.exe`, que exporta,
inspeciona e importa esse arquivo.

## Decisão

- **O export mora no repo** em `windows/windhawk/backup.json`, como o Windhawk o grava. Ele não
  tem segredos e o git mostra o que mudou entre um export e outro.
- **O CLI faz o trabalho.** Importar é `windhawk-cli data import --yes --confirm-app-restart`:
  ele instala os mods do repositório do Windhawk e aplica as configurações. Roda elevado (o
  Windhawk guarda os dados em HKLM e ProgramData), então a saída volta por arquivos
  (`Start-Process -RedirectStandardOutput`). Mods instalados e fora do export ficam como estão
  (é o comportamento do CLI).
- **Comparar é exportar.** O estado da máquina vem de um `data export` sem elevação para um arquivo
  temporário, comparado mod a mod com o do repo: faltando, versão, ativado/desativado e cada
  configuração. Números são comparados pelo valor.
- **Uma dock só: a do "Ilha e Dock estilo Mac".** O Taskbar Styler saiu (script, estilo e ícone).
  `taskbar-dock` muda só o `taskbarMode` do `mac-island-dock` com `mod settings set`, instalando
  o mod antes se faltar, no mesmo script elevado; o modo anterior fica no diretório de estado do
  envsetup para `--restore`. O estado vem de `mod settings get`, sem elevação.
- **Método `windhawk` no catálogo.** `windhawk-mods` conta como feito quando nenhum mod do export
  falta ou está diferente; aplicar é importar. Ele vem depois do item `windhawk` (winget
  `RamenSoftware.Windhawk`) na etapa Aparência.

## Consequências

- Só funciona com o Windhawk 2: o 1.x não tem o CLI, e a aba avisa quando ele não é achado.
- Importar baixa os mods do repositório do Windhawk; sem internet, falha. Um export com
  `--offline` levaria o código-fonte junto, mas não é o que o app grava.
- Ler, comparar, exportar e ler o modo da dock rodaram contra o Windhawk 2.0 alpha 8 desta
  máquina. A importação e a troca de modo da dock (elevadas) não rodaram de verdade; o script foi
  conferido com `data inspect` no lugar de `import`, e gravar sem elevação falha com "Acesso
  negado", o que confirma que a elevação é necessária.
