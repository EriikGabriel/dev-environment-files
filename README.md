# envsetup

O **envsetup** monta o meu ambiente numa máquina recém-formatada. Ele detecta o sistema, lê uma
descrição do ambiente (apps, dotfiles, pastas e, no Windows, aparência e sistema) e aplica só o
que falta. Tem um app desktop (Tauri + React) e uma linha de comando sobre o mesmo motor em Go.

| Ambiente        | Máquina         | O que monta |
| --------------- | --------------- | ----------- |
| Desenvolvimento | CachyOS e Arch  | niri + Noctalia, fish + starship, kitty, Zed |
| Desenvolvimento | Ubuntu e Debian | GNOME + WhiteSur, zsh + powerlevel10k, WezTerm, VS Code |
| Jogos           | Windows         | instalação automática (Ventoy), winget e Ninite, políticas de grupo, barra de tarefas e ícones, mods do Windhawk, Playnite, RetroArch e jogos |

## Começar

1. **Instale o app** pelo [último release](https://github.com/EriikGabriel/dev-environment-files/releases/latest).
   Ele já traz o motor e os dados: não precisa de Go, Node, Rust nem git.
   - Windows (PowerShell): `irm https://github.com/EriikGabriel/dev-environment-files/releases/latest/download/install.ps1 | iex`
   - Ubuntu e Arch: `curl -fsSL https://github.com/EriikGabriel/dev-environment-files/releases/latest/download/install.sh | bash`
2. **Na primeira execução**, escolha "Usar os dados incluídos" (uma cópia editável em
   `Documentos\envsetup` ou `~/envsetup`) ou "Escolher pasta…" para usar um clone deste repo.
3. **Configurar**: revise as etapas do perfil, marque o que quer e clique em Aplicar.

Não é o dono? Dá para usar com as configurações dele, sem os saves e as contas dele: veja
[para outra pessoa](docs/formatacao.md#para-outra-pessoa-instalar-com-as-configurações-do-dono).

Detalhes em **[docs/uso.md](docs/uso.md)**: as abas do app, a linha de comando, [como formatar o PC](docs/formatacao.md) (o que volta e o que não volta) e como cada
recurso do Windows funciona.

## Mudar o ambiente

Tudo o que o envsetup faz vem dos arquivos da pasta de dados:

```
config/apps.yaml           catálogo: como instalar cada app em cada sistema
config/profiles/*.yaml     um perfil por máquina, dividido em etapas
config/policies.yaml       políticas de grupo do Windows
config/layout.yaml         barra de tarefas e menu Iniciar do Windows
config/icons.yaml          ícones dos atalhos do Windows
config/playnite.yaml       Playnite portable e o HD com a biblioteca e as mídias
config/retroarch.yaml      RetroArch portable: versão, cores e o HD com BIOS e jogos
windows/                   instalação automática, backup do Windhawk, pack de ícones,
                           configurações e saves do Playnite e do RetroArch (sem senhas)
dotfiles/                  os arquivos de configuração que vão para a home
```

O que cada um controla, os campos, exemplos e como editar (pelo app, por comando ou à mão) estão
em **[docs/configuracao.md](docs/configuracao.md)**.

## Segredos

Não guarde chaves nem senhas nesses arquivos: tudo neles vai para o instalador, e num clone vai
para o git. As cópias daqui usam marcadores (`<FIGMA_API_KEY>`, `<PASSWORD>`), e o
`autounattend.xml` não tem senha. Como os dotfiles viram symlinks, preencher o valor real no
arquivo da home o grava na pasta de dados.

## Desenvolvimento

```bash
cd app && npm install && npm run tauri dev    # o app, compilando o motor antes
go test ./...                                 # o motor
```

Mapa do código, testes, o contrato entre o app e o motor e como os dados entram no instalador:
**[docs/desenvolvimento.md](docs/desenvolvimento.md)**. As decisões de arquitetura estão em
[docs/decisions/](docs/decisions/README.md).

## To-do

- [x] Perfil de jogos no Windows (backends winget/scoop)
- [x] Repositórios de terceiros declarativos (docker-ce, NodeSource)
- [x] Identidade git, chave SSH e `gh auth login`
- [x] `webkit2gtk` e FUSE 2 no catálogo
- [x] Emulador: RetroArch portable e jogos (configurações e saves no git, programa baixado, BIOS e jogos no HD)
- [x] Salvar o RetroArch (`D:\envsetup`)
- [x] Salvar o Playnite (`D:\envsetup`) e centralizar os jogos em `D:\Games`
- [x] Aba Backup: passo a passo para formatar
- [ ] Testar os scripts do GNOME numa sessão real

## Tempo de execução das etapas

- Instalação da ISO do Windows: 12 min
- Instalação dos drivers e updates do Windows (depende do tamanho do update): ~30 min
- Configuração do sistema através do app: ~6 min
