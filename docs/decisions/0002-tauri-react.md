## Contexto

A TUI resolvia bem o bootstrap, mas o uso real inclui gerenciar as configs: arrastar
arquivos e imagens, editar, e guardar dotfiles no repo. Num terminal isso é ruim. Uma
interface web local também não serve, porque o navegador entrega o conteúdo de um arquivo
arrastado mas não o **caminho**, e capturar `~/.config/algo` para virar symlink precisa dele.

## Decisão

- **Tauri 2 + React (TypeScript, Vite)** em `app/`. O drag & drop nativo do Tauri entrega
  caminhos reais; o seletor de arquivos também.
- **O motor continua em Go e roda como sidecar.** O Tauri empacota o binário `envsetup`
  (`bundle.externalBin`) e o chama com `--json`. O Rust não tem lógica de negócio: só
  valida o subcomando, sobe o processo e repassa a saída. Assim o motor testado não é
  reescrito, e CLI, TUI e GUI usam exatamente o mesmo código.
- **A CLI e a TUI continuam**, para SSH, máquina sem sessão gráfica e bootstrap sem o app.

## Contrato Go ↔ app

| Comando                                            | Saída                                                                                             |
| -------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| `info --json`                                      | distro, kernel, desktop, shell, CPU, memória, gerenciador de pacotes                              |
| `profiles --json`                                  | perfis, com o ambiente (`dev`/`gaming`) e qual corresponde à máquina                              |
| `steps --json [-p perfil]`                         | etapas do perfil: apps com estado, opções de escolha, arquivos e pastas                           |
| `plan --json [--apps a,b] [--choose etapa=opção]`  | itens (`app`, `dir`, `file`, `vscode`) com estado e etapa de origem, mais os indisponíveis        |
| `apply --json [--sudo-password-stdin]`             | NDJSON: `step` (com etapa e duração), `log` (com o índice do item), `done` (com o caminho do log) |
| `add <caminho> --json [--group g] [--copy]`        | `{src, dest, mode}`                                                                               |
| `show <caminho> --json`                            | conteúdo de um arquivo do repo, listagem de pasta ou imagem; recusa caminhos fora do repo         |
| `set-dirs --step e [--option o] [pasta...] --json` | grava a lista de pastas no YAML do perfil                                                         |

O Rust só aceita esses subcomandos (lista `ALLOWED` em `app/src-tauri/src/lib.rs`).

Os tipos do lado TypeScript (`app/src/types.ts`) espelham `cmd/envsetup/json.go` e
`internal/engine`; mudar um exige mudar o outro.

## Privilégios sem terminal

O app não tem TTY, então `sudo` não consegue pedir senha. Com `--sudo-password-stdin` o
sidecar lê a senha da primeira linha do stdin, valida com `sudo -v` e coloca no início do
`PATH` um wrapper `sudo` temporário que alimenta o `sudo -S`. Isso vale para qualquer
filho (scripts `sh -c`, yay/paru que chamam `sudo` sozinhos). A senha existe só no
ambiente do processo, que apenas o mesmo usuário lê, e some ao fim da execução.

## Consequências e riscos

- **Linux:** o app exige `webkit2gtk-4.1` em runtime. Numa instalação mínima ele pode faltar;
  o caminho previsto é a CLI instalar essa dependência antes de abrir o app, mas o `webkit2gtk`
  ainda não está no catálogo (`config/apps.yaml`), então hoje isso é manual.
- **Windows:** o WebView2 já vem no Windows 11. O sidecar é compilado por alvo
  (`app/scripts/build-sidecar.mjs`); o cross-compile do app Tauri é frágil, então o `.exe`
  deve sair de uma máquina ou CI Windows. Ainda não testado.
- **Arquivos grandes** (imagens, binários) inflam o git; considerar Git LFS se virarem muitos.
- O app precisa saber onde está o repo (`config/` e `dotfiles/`). Ele detecta a partir do
  diretório atual ou do executável, e guarda a escolha do usuário.
