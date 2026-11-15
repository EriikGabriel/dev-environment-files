## Contexto

A primeira versão tinha um perfil único (`dev`) com uma lista plana de apps e arquivos, e a
mesma lista valia para qualquer distro. Na prática os ambientes finais são diferentes: no
Ubuntu o alvo é o dos scripts antigos (GNOME + WhiteSur, zsh, WezTerm, VS Code) e no CachyOS é
o ambiente atual (niri + Noctalia, fish, kitty, Zed). Além disso a configuração precisa ser
feita em passos, com escolhas, e não num bloco só.

## Decisão

- **Um perfil por tipo de máquina** em `config/profiles/`, escolhido por `match` (ID da distro,
  depois `ID_LIKE`; `windows` para o Windows). O catálogo `config/apps.yaml` continua único e
  compartilhado.
- **Cada perfil é uma lista ordenada de etapas.** Uma etapa pode ter:
  - `apps` (começam marcados) e `optional` (começam desmarcados);
  - `options` com `default`: escolha única (shell, terminal, editor, estrutura de pastas);
    `none` é sempre válido e pula a escolha;
  - `files`: dotfiles, opcionalmente amarrados a um app com `app:`;
  - `dirs`: pastas a criar.
- **Ordem de execução:** todos os apps na ordem das etapas, depois pastas, depois arquivos e por
  fim extensões do VS Code. Assim um instalador não sobrescreve um dotfile recém-implantado.
- **Ajustes de desktop são apps do tipo `script`** com `check` (GNOME: gsettings, extensões,
  tema, dock). Não foi criado um tipo novo de passo para isso.
- **As escolhas do usuário não vão para o repo.** O app guarda a seleção localmente e a passa
  ao motor por `--apps` e `--choose`. A exceção é a lista de pastas (`dirs`), que é
  configuração do ambiente e por isso é gravada no perfil.
- **Edição do YAML por texto.** `add` e `set-dirs` alteram só a linha necessária, em vez de
  reserializar o arquivo, para preservar comentários e manter os diffs do git pequenos.

## Desktop e preview

Cada perfil tem uma seção `desktop:` (tipo, wallpaper, tema de ícones, cores, dock e painel). Ela
alimenta o preview do app e é a fonte única da **ordem da dock**: o passo `dock` do `apply`
escreve essa ordem no desktop de verdade (Noctalia: `pinned` no `config.toml`, pelo symlink,
então o arquivo do repo muda; GNOME: `favorite-apps`, só com apps instalados). O preview usa os
`.desktop` e o tema de ícones instalados na máquina, então o que ainda não está instalado
aparece como um quadrado colorido. O wallpaper pode ser um arquivo do repo, um caminho em `~`
ou uma URL https (baixada uma vez para o cache).

## Consequências

- Adicionar uma distro com ambiente próprio é criar um arquivo de perfil; adicionar um app é
  uma entrada no catálogo e o id numa etapa.
- `--apps` filtra apenas os itens marcáveis; os apps de uma opção escolhida entram sempre.
- `set-dirs` exige que `dirs` esteja numa linha só (`dirs: [a, b]`).
- Os scripts do GNOME dependem da sessão em execução e não são testáveis em contêiner.
- O perfil `gaming-windows` existe só como marcador; o motor recusa plataformas fora das
  famílias Debian e Arch.
