## Contexto

O app desktop virou a interface principal e a TUI de 0001 ficou parada: era a única parte que
usava bubbletea e lipgloss. Para usar o executável era preciso ter um clone do repositório,
porque o app procura `config/apps.yaml` numa pasta do disco. O código também tinha crescido por
adição: `winsetup` (Windows) ao lado de `windows/` (dados), um `cmd/envsetup/windows.go` com mil
linhas e cinco assuntos, e o `engine` misturando Linux e Windows sem indicar qual é qual.

## Decisão

- **Go continua.** Ele não é pré-requisito de quem usa o app: vira um binário estático que vai
  no instalador. Trocar por Rust só tiraria o Go da compilação, ao custo de reescrever o motor.
- **Sem TUI.** `envsetup` sem subcomando mostra a ajuda. A CLI continua para scripts e SSH.
  Substitui a decisão 1 de [0001](0001-tui-go.md).
- **Os dados vão no instalador.** `config/`, `dotfiles/` e `windows/` entram em `resources/data`.
  Na primeira execução o app oferece copiá-los para `Documentos\envsetup` (Windows) ou
  `~/envsetup` (Linux), uma pasta editável, ou usar um clone. A cópia nunca sobrescreve um
  arquivo; "Atualizar dados" só acrescenta o que uma versão nova traz e não aparece num clone git.
- **Organização.** Os recursos do Windows ficam em `internal/windows/<recurso>`; `winsetup` se
  dividiu em `install` (arquivo de resposta e pendrive) e `ninite`. No `engine`, arquivos `win_*` e
  `linux_*` (prefixo, porque sufixos `_windows.go` viram restrição de build). A CLI tem um arquivo
  por assunto. No front, `components/windows/` e `components/linux/`, `types/` por assunto e o hook
  `useApplyRun`.
- **`legacy/` saiu.** O tema GTK que o perfil do Ubuntu usa foi para `dotfiles/gtk/`; os scripts
  bash continuam no histórico do git.
- **Nomes de subcomandos e campos JSON não mudaram:** são o contrato com o app.

## Consequências

- O instalador fica ~35 MB maior (o pack de ícones é a maior parte).
- Tudo o que está nas três pastas de dados vai para o instalador, inclusive o nome da conta do
  `autounattend.xml` e caminhos da home em alguns dotfiles.
- Quem usa a cópia em Documentos não tem histórico das próprias mudanças; para isso, use um clone.
- Os dotfiles são symlinks para a pasta de dados: mover a pasta quebra os links até aplicar de novo.
- Caminhos de imagem escolhidos dentro da pasta de dados (overrides de ícones) passam a ser
  gravados relativos a ela, para valerem em outra máquina.
