## Contexto

Cada versão nova exigia desinstalar o app e rodar o instalador da release de novo. O Tauri tem um
plugin de atualização, mas ele pede um par de chaves de assinatura (a chave privada nos secrets do
workflow) e um `latest.json` publicado na release. Nada disso existe, e o instalador do Windows
não é assinado (Authenticode).

## Decisão

- **O motor em Go faz a parte de rede** (`internal/appupdate`, subcomandos `app-update-check` e
  `app-update-download`), sem nova dependência no Rust. Ele pergunta à API do GitHub qual é a
  última release (nunca rascunho nem pré-release) e compara a versão em números (`0.1.10` > `0.1.9`).
- **O instalador é o da versão da release.** Uma release pode carregar arquivos de um build anterior
  (a v0.2.0 trouxe `envsetup_0.1.10_…` e `envsetup_0.2.0_…`): só vale `envsetup_<versão>_…-setup.exe`.
- **A integridade vem do SHA-256 que o GitHub guarda de cada arquivo** (`digest` da API). O download
  só vale se o endereço é um arquivo das releases deste repositório, se o tamanho e o hash batem e
  se a release tem hash; senão o arquivo é apagado e o instalador não roda. Sem assinatura própria,
  isso protege contra um download corrompido ou trocado no caminho, não contra a conta do GitHub
  comprometida.
- **O app só roda um arquivo que ele mesmo baixou:** o comando Rust `install_update` aceita apenas
  um `envsetup*-setup.exe` direto dentro de `%TEMP%\envsetup-update` (o caminho vem do front-end,
  então não é confiado). Ele inicia um PowerShell oculto, que sobrevive ao app: espera o instalador
  silencioso (`/S` do NSIS, sem cliques) e abre o app de novo, que é o novo se deu certo e o antigo
  se não. O app fecha antes, para o instalador poder trocar os arquivos.
- **Fluxo:** uma procura silenciosa 2,5 s depois de abrir (sem rede, nada aparece), um aviso no topo
  ("Atualizar agora", "Detalhes", "Depois"), e a seção Atualizações em Sobre ou "Procurar atualização"
  na busca (Ctrl K). Num build de desenvolvimento (`target\`) a instalação é recusada.
- **Linux:** o app avisa que há versão nova e mostra o comando do `install.sh`; ele não instala
  pacotes sozinho.

## Consequências

- O app instalado se atualiza sozinho no Windows, mas só enquanto o instalador for per-user (o do
  Tauri por padrão): um instalador que pede elevação não roda em silêncio e o app reabre antigo.
- Sem assinatura do instalador, um atacante que publique na conta do GitHub entrega código. Assinar
  o instalador (certificado de código) e o `latest.json` do plugin do Tauri seria o passo seguinte.
- O GitHub limita a API sem login (60 por hora por rede); a procura é uma por abertura do app.
- A pasta de dados (`Documentos\envsetup`) não é trocada na atualização: arquivos novos entram com
  "Atualizar dados", como antes.
