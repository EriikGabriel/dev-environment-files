# Ícones do macosicons.com

Ícones escolhidos à mão para os apps que o tema MacTahoe não tem, ou desenha
errado. `go run ./cmd/iconpack` copia esta pasta por cima de
`windows/icons/mactahoe` (o pack que o app usa), então eles aparecem na
biblioteca da aba Preview e são aplicados pelo nome do atalho, como os outros.

Cada um é o PNG de 1024 px do `.icns` da página, recortado e reduzido para
256 px com a mesma margem dos ícones do MacTahoe.

| Arquivo | Atalho do Windows | Origem |
|---|---|---|
| `dia.png` | Dia (o navegador da The Browser Company; o `dia.png` do MacTahoe era o editor de diagramas do GNOME) | https://macosicons.com/icon/dia-hxUwoDa0xe |
| `playnite.png` | Playnite | https://macosicons.com/?icon=TYLkUIi7Uk |
| `retroarch.png` | RetroArch | https://macosicons.com/?icon=xWBRy8t6Dq |

Para trocar ou acrescentar um: salve o PNG aqui com o nome do atalho (em
minúsculas, como `hydra-launcher.png`), rode `go run ./cmd/iconpack` (ou copie
o arquivo para `windows/icons/mactahoe` à mão) e aplique os ícones na aba
Preview.
