# Decisões de arquitetura

| # | Decisão | Situação |
|---|---|---|
| [0001](0001-tui-go.md) | Motor em Go com TUI | TUI substituída por 0002 e removida em 0008 |
| [0002](0002-tauri-react.md) | App desktop em Tauri + React, motor como sidecar | em vigor |
| [0003](0003-perfis-e-etapas.md) | Perfis divididos em etapas | em vigor |
| [0004](0004-windows-e-jogos.md) | Perfil de jogos no Windows (winget, PowerShell, Steam) | em vigor |
| [0005](0005-instalacao-windows-e-politicas.md) | Instalação automática do Windows e políticas de grupo | em vigor |
| [0006](0006-playnite-portable.md) | Playnite portable por snapshot | em vigor |
| [0007](0007-windhawk-backup.md) | Mods do Windhawk pelo backup do Windhawk 2 | em vigor |
| [0008](0008-sem-tui-e-dados-no-instalador.md) | Sem TUI, dados no instalador, código reorganizado | em vigor |
| [0009](0009-fixados-da-barra-do-usuario.md) | Fixados da barra gravados como do usuário, sem a política "Start layout" | em vigor |
| [0010](0010-atualizacao-pelo-app.md) | O app se atualiza sozinho (consulta o GitHub, confere o SHA-256, instala em silêncio) | em vigor |

Cada arquivo tem Contexto, Decisão e Consequências. Uma decisão nova que muda uma antiga ganha
um número novo e a antiga é marcada aqui, sem ser reescrita.
