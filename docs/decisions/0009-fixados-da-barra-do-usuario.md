## Contexto

Os apps da barra eram entregues ao Windows num `TaskbarLayout.xml` apontado pela política
"Start layout" (`StartLayoutFile`, em HKLM), com `PinListPlacement="Replace"`. O Windows relê esse
arquivo a cada login e, nos builds recentes do Windows 11 (testado no 26300), descartava os fixados
que não conseguia resolver: depois de sair e entrar só sobravam o Explorador e um ou outro app. A
cada reaplicação ele copiava os atalhos de novo para `User Pinned\TaskBar`, então a pasta foi
acumulando cópias (`Brave (2..4).lnk`, `File Explorer (2..6).lnk`). Um guarda iniciado no login
tentava devolver o layout e não conseguia.

Fixar pelo caminho que os programas usavam (`IPinnedList3::Modify`, o verbo `taskbarpin`) não
serve: fora do processo do Explorer o Windows 11 responde OK e não fixa nada. A Microsoft só
documenta o XML por política, e fóruns (NTLite) relatam que ele não funciona desde 2023.

## Decisão

- **Os fixados são do usuário, gravados como o próprio Explorer os grava:** uma cópia do atalho
  do menu Iniciar em `%APPDATA%\Microsoft\Internet Explorer\Quick Launch\User Pinned\TaskBar` e
  a lista em `HKCU\...\Explorer\Taskband`. Em `Favorites` vai, para cada um e em ordem, `0x00`,
  o tamanho e a lista de ids do item sob a pasta "User Pinned" (`{1f3427c8-…}`): o item da pasta
  TaskBar e o do atalho, este com os três blocos que o Explorer confere (`0xbeef002b` com a
  hora, `0xbeef001d` com o id do app e `0xbeef001e` com `UserPinned`). A lista termina em `0xFF`.
  `FavoritesResolve` tem um link para cada atalho. O Explorer lê isso quando inicia.
- **Todo fixado leva o id do app** (o do atalho, o que o `Get-StartApps` mostra, ou o caminho do
  programa com a pasta conhecida escrita como id). Um registro que o Explorer não aceita o faz
  descartar aquele fixado e todos os que vêm depois.
- **Substitui o que estava lá**, inclusive as cópias antigas, e o `desktop.ini` da pasta só fica
  com os nomes traduzidos dos atalhos que estão nela.
- **Confere depois de reiniciar o Explorer**, lendo a lista da barra (`IPinnedList3::EnumObjects`,
  que funciona fora do Explorer), e tenta uma segunda vez se algum faltar.
- **O que as versões antigas deixaram sai:** a política `StartLayoutFile` (só se aponta para o
  arquivo do envsetup), o XML, o guarda e o valor `envsetup-taskbar` do `Run`.

## Consequências

- Nada reaplica os fixados no login: eles ficam até o usuário mudar, como os que ele fixa à mão.
  Mudar a ordem à mão não é desfeito.
- O formato do `Favorites` não é documentado. Se um build futuro mudar, o passo avisa ("not on the
  taskbar after Explorer restarted") em vez de deixar a barra pela metade em silêncio.
- Um app MSIX sem atalho no menu Iniciar não é fixado (o Dia ganha o atalho pelo item `dia`).
- Os atalhos fixados são cópias dos do menu Iniciar, então já levam o ícone do pack; o passo de
  ícones também troca os da pasta TaskBar.
