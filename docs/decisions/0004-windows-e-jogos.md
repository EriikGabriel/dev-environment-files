## Contexto

O perfil de jogos no Windows era só um marcador: o motor recusava a plataforma, todo script
rodava com `sh -c` e a elevação dependia do `sudo`. O objetivo é que ele funcione como os
perfis de desenvolvimento (etapas, itens marcáveis, revisão, aplicar) e que os apps possam ser
trocados sem editar YAML à mão.

## Decisão

- **winget é o método nativo do Windows.** `names: { windows: Valve.Steam }` basta; `winget`
  pode ser escrito explicitamente e `scoop` é um método à parte. O winget é chamado com o ID
  exato, em modo silencioso e aceitando os termos.
- **`"*"` não vale no Windows.** As entradas curinga do catálogo são nomes de pacote e scripts
  de Linux, então o Windows só usa o que está na chave `windows`.
- **Scripts em PowerShell.** `check`, `source` e `post` rodam com `powershell -NoProfile
  -NonInteractive`; não existe `$SUDO`, e cada instalador pede a própria elevação (UAC). O app
  não mostra o diálogo de senha no Windows.
- **Método `steam`.** `names` é o AppID. O item está instalado quando existe
  `appmanifest_<id>.acf` em alguma biblioteca listada em `libraryfolders.vdf`. Instalar abre
  `steam://install/<id>`: a Steam mostra a própria janela e o app precisa estar na conta, então
  o passo só inicia a instalação e passa a contar como instalado numa execução seguinte.
- **AMD Software por script.** O pacote não existe no winget (o `AMD.AMDSoftwareCloudEdition` é
  para VMs do Azure). O item lê a página de drivers da AMD, baixa o instalador de auto-detecção
  e o abre; se o link não for encontrado, abre a página e falha com uma mensagem clara.
- **Dotfiles por cópia.** No Windows o modo padrão de `files` é `copy`, porque criar symlink
  exige o modo desenvolvedor ou um processo elevado.
- **Adicionar e remover apps pela interface.** `add-app` cria a entrada no catálogo (para a
  família da máquina em uso) e põe o id na etapa; `remove-app` tira o id da etapa e mantém o
  catálogo. Valem para todos os perfis.

## Consequências

- Os IDs do winget foram conferidos no repositório `microsoft/winget-pkgs`, mas nada disso
  rodou numa máquina Windows: a validação foi por testes com runner falso e compilação cruzada.
- O link do instalador da AMD depende do HTML do site; se ele mudar, o item passa a abrir a
  página para download manual.
- O app da NVIDIA não está no winget e ficou de fora.
- Não há busca de pacotes dentro do app: o ID do winget (ou o AppID da Steam) é digitado.
- Um app criado por `add-app` só tem a chave da família onde foi criado; para outra família é
  preciso completar a entrada no catálogo.
