# Desenvolvimento

## Ferramentas

Só para compilar: **Go** (o motor), **Node 20+** (o front-end) e **Rust** (o app Tauri). No Linux,
também `webkit2gtk-4.1`. Quem usa o instalador não precisa de nenhuma delas: o motor vira um
`envsetup` estático (sem cgo) que vai dentro do app.

```bash
cd app
npm install
npm run tauri dev      # abre o app; compila o motor antes (npm run sidecar)
npm run tauri build    # gera o instalador, com o motor e os dados dentro
```

`npm run sidecar` (`app/scripts/build-sidecar.mjs`) compila `cmd/envsetup` para
`app/src-tauri/binaries/envsetup-<triple>`, o nome que o Tauri espera em `externalBin`. Rode de novo
depois de mudar o Go, senão o app usa o binário antigo.

## Testes

```bash
go test ./...                          # motor e CLI
GOOS=linux go vet ./...                # confere que os arquivos de Linux continuam compilando no Windows (e vice-versa)
cd app && npx tsc --noEmit             # tipos do front-end
cd app/src-tauri && cargo test --lib   # a cópia dos dados (nunca sobrescreve)
```

No Windows, 4 testes que criam symlinks são pulados, a não ser com o Modo de Desenvolvedor
ativado. Os testes acham a pasta do repo com `testutil.RepoRoot(t)`, não com caminhos relativos.

## Mapa do código

```
cmd/envsetup/          a CLI, um arquivo por assunto
  main.go              o comando raiz e a lista de grupos de comandos
  flags.go root.go     flags comuns, achar a pasta de dados e o perfil
  machine.go           detect, info, profiles, steps
  apply.go             plan, apply (e o modo --json que o app usa)
  edit.go              add, show, set-dirs, desktop, set-dock, add-app, remove-app
  install.go policies.go layout.go icons.go windhawk.go playnite.go   (Windows)
  json.go              formatos JSON comuns
cmd/iconpack/          ferramenta que refaz windows/icons/mactahoe a partir do tema MacTahoe

internal/
  config/              carrega e edita apps.yaml e os perfis
  engine/              perfil + máquina -> passos idempotentes, e a execução deles
                       sem prefixo: núcleo; linux_*: só Linux; win_*: só Windows
  backend/             gerenciadores de pacote (apt, pacman, AUR, flatpak, winget, scoop)
  platform/            detecção do sistema, informações da máquina, abrir links
  windows/             recursos do Windows, um pacote cada:
    install/           autounattend.xml e o pendrive Ventoy
    ninite/            lista e links do Ninite
    policy/ layout/ icons/ playnite/ windhawk/
  testutil/            ajudas dos testes

app/
  src-tauri/src/lib.rs     ponte: roda o envsetup, acha e copia a pasta de dados
  src/App.tsx              navegação entre as abas
  src/api.ts               uma função por subcomando
  src/types/               formatos JSON, um arquivo por assunto
  src/useApplyRun.ts       estado de uma execução do apply
  src/components/          abas comuns; windows/ e linux/ as específicas
```

**Prefixos, não sufixos.** No `engine`, arquivos de Windows começam com `win_` e os de Linux com
`linux_`. Nunca use `_windows.go` ou `_linux.go` no fim do nome: o Go entende como restrição de
build e ignora o arquivo ao compilar para o outro sistema (e o app compila os dois).

**Por que os recursos do Windows ainda têm arquivos no `engine`.** As partes que rodam algo na
máquina (`ApplyPolicies`, `SavePlaynite`, `ImportWindhawk`…) são métodos de `engine.Env` porque
usam o runner, o PowerShell e a elevação compartilhados, e porque os itens de catálogo (`policies`,
`playnite`, `windhawk`) os chamam. Os pacotes de `internal/windows/` têm o resto: formatos,
validação, scripts gerados e leitura de arquivos, sem rodar nada.

## O contrato entre o app e o motor

O app chama o `envsetup` com `--json` e lê a resposta. Três pontos precisam andar juntos:

1. o subcomando e as tags JSON em `cmd/envsetup/*.go`;
2. os tipos em `app/src/types/` e a função em `app/src/api.ts`;
3. a lista `ALLOWED` em `app/src-tauri/src/lib.rs`: o app só roda os subcomandos dela.

Renomear um subcomando ou um campo exige mudar os três. Para dar um nome melhor a um comando sem
quebrar nada, acrescente um `Aliases` no cobra e o nome novo na `ALLOWED`.

## Os dados no instalador

`bundle.resources` em `app/src-tauri/tauri.conf.json` copia `config/`, `dotfiles/` e `windows/`
para `resources/data` do instalador. O prefixo `data/` é necessário: no Windows os recursos ficam
na pasta do executável, e sem ele o app acharia `config/apps.yaml` ali, numa pasta só de
leitura, e a usaria como pasta de dados. Na primeira execução, `seed_data` (em `lib.rs`) copia
para `Documentos\envsetup` / `~/envsetup` e grava `.envsetup-data` com a versão; `update_data`
acrescenta o que faltar, sem sobrescrever. Tudo o que estiver nessas três pastas vai para o
instalador, então não guarde lá nada que não deva ser distribuído.

## Decisões

O porquê de cada escolha está em [decisions/](decisions/README.md).
