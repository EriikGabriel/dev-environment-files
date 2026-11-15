## Contexto

Os scripts bash em `legacy/` só servem Ubuntu/GNOME: `apt` embutido em tudo, nomes de
pacote espalhados em 20+ loops, sem detecção de distro nem formato de config. O objetivo é
trocar de distro (Ubuntu, Arch) sem reescrever nada e ter o mesmo para o Windows de jogos.

## Decisão

1. **TUI, não app desktop.** O ambiente precisa subir numa máquina recém-formatada, às
   vezes só com TTY ou por SSH. Um app gráfico (Tauri, Electron, GTK) exige runtime e
   dependências antes do bootstrap, que é justamente o que se quer evitar.
2. **Go com motor próprio.** Um binário estático (`CGO_ENABLED=0`) roda em Linux e, depois,
   Windows via cross-compile, sem instalar nada antes. Estrutura: core (biblioteca) + CLI +
   TUI. Uma GUI futura seria só mais um front-end sobre o mesmo core.
3. **Config declarativa em YAML**, com validação estrita (chave desconhecida é erro).
4. **Evoluir este repo**, mantendo os scripts antigos em `legacy/` até serem migrados.

## Alternativas consideradas

| Opção                                              | Por que não                                                                                                                        |
| -------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| TUI/CLI por cima de chezmoi + Ansible + winget DSC | Três formatos, mais dependências antes do bootstrap; a lógica por distro ficaria espalhada.                                        |
| Nix + home-manager                                 | Reprodutível de verdade, mas curva alta, sem Windows, e muda como Ubuntu/Arch são usados.                                          |
| Python (Textual) ou Rust (Ratatui)                 | Python precisa de interpretador no alvo; Rust serviria, mas Go compila mais rápido e a stdlib (exec, http, os) cobre o necessário. |

> As referências abaixo (chezmoi, Ansible, dotbot, Brewfile, winget configuration, Scoop,
> Omakub/Omarchy) foram citadas de memória: a busca web falhou na sessão que gerou este
> documento. Conferir versões e manutenção antes de se apoiar em qualquer uma delas.

## Consequências

- Um app declara `names`/`method`/`source` por distro numa tabela só; adicionar uma distro
  é acrescentar uma chave.
- Cada passo avalia o estado antes de agir, então `apply` pode rodar várias vezes. Os
  `post` rodam sempre e precisam ser idempotentes.
- No Arch o refresh é `pacman -Syu` (upgrade completo), porque `-Sy` seguido de install é
  um upgrade parcial, sem suporte.
- No modo TUI o terminal pertence ao Bubble Tea: o `sudo -v` é feito uma vez antes e os
  comandos seguem com `sudo -n`. Se a credencial expirar, o passo falha em vez de travar.
- Windows ainda não existe: `BuildPlan` recusa a plataforma. A interface `Backend` e o campo
  `method` já comportam winget/scoop.

## Limitações conhecidas do MVP

- Repositórios de terceiros (docker-ce, NodeSource) não são modelados; usa-se o pacote da
  própria distro.
- O Chrome só tem build `amd64` (falha em arm64); o método AUR exige yay ou paru já instalado.
- O `post` do apt de `fd` e `bat` roda em todo `apply`.
