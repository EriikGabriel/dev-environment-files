#!/bin/bash

# Clear terminal
clear

# Funções para exibir mensagens coloridas
green() { echo -e "\033[32m$1\033[0m"; }
blue() { echo -e "\033[34m$1\033[0m"; }
yellow() { echo -e "\033[33m$1\033[0m"; }
red() { echo -e "\033[31m$1\033[0m"; }

# Verifica se está rodando como root
if [[ $EUID -ne 0 ]]; then
    red "❌ Este script deve ser executado como root ou com sudo!"
    exit 1
fi

# Função para aguardar a liberação do lock do apt
wait_for_apt_lock() {
    local timeout=60  # Tempo máximo de espera (segundos)
    local wait_time=0

    while sudo lsof /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/lib/apt/lists/lock >/dev/null 2>&1; do
        if (( wait_time >= timeout )); then
            red "⏳ O lock do apt demorou demais para ser liberado. Tentando remover..."
            sudo rm -f /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/lib/apt/lists/lock
            sudo dpkg --configure -a
            break
        fi
        yellow "⏳ Aguardando liberação do lock do apt... (${wait_time}s)"
        sleep 2
        (( wait_time += 2 ))
    done
    green "✅ Lock do apt liberado!"
}

# Função para instalar pacotes se não estiverem instalados
install_package() {
    if ! dpkg -l | grep -q "^ii  $1 "; then
        wait_for_apt_lock
        sudo apt install -y "$1" && green "✅ $1 instalado com sucesso!" || red "❌ Falha ao instalar $1!"
    else
        blue "✅ $1 já está instalado!"
    fi
}

# Função para exibir barra de progresso fixa com caracteres especiais
progress_bar() {
    local total_steps=$1
    local current_step=$2
    local width=40
    local completed=$((width * current_step / total_steps))
    local remaining=$((width - completed))

   if [ "$clear_option" == "y" ]; then
        clear
    fi

    printf "\r["
    printf "%0.s█" $(seq 1 $completed)   # Parte preenchida
    printf "%0.s░" $(seq 1 $remaining)   # Parte restante
    printf "] %d%% - %s" $((100 * current_step / total_steps)) "$3"
    echo ""
}

# Endereço do diretório home do usuário original
USER_HOME=$(eval echo ~$SUDO_USER)

# Total de passos (ajustado para o número total de etapas)
TOTAL_STEPS=28
CURRENT_STEP=0

yellow "🚀 Iniciando configuração do sistema..."

# Perguntar se deseja ativar a opção de clear
clear_option="y"
read -p "Deseja ativar a opção de clear? (Y/n): " clear_option
if [ "$clear_option" == "n" ]; then
    blue "> Opção de clear desativada!"
else
    green "> Opção de clear ativada!"
    clear_option="y"
fi


# Atualizar sistema
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🛠️ Atualizando sistema..."
wait_for_apt_lock
sudo apt update && sudo apt upgrade -y && sudo apt dist-upgrade -y
green "\n✅ Sistema atualizado!"

# Instalar ferramentas essenciais
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🧰 Instalando ferramentas essenciais..."
for package in build-essential curl wget git vim zsh; do
    install_package "$package"
done

# Criar arquivo de config do Zsh
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔧 Configurando Zsh..."
if [ ! -f "$USER_HOME/.zshrc" ]; then
    cp ./zsh/.zshrc $USER_HOME/.zshrc
    chown $SUDO_USER:$SUDO_USER $USER_HOME/.zshrc
    green "✅ Configuração do Zsh criada!"
else
    blue "✅ Arquivo de configuração do Zsh já existe!"
fi

# Instalar Snap
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔌 Instalando Snap..."
install_package snapd

# Instalar Docker se não estiver instalado
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🐳 Instalando Docker..."
if ! dpkg -l | grep -q "docker-ce"; then
    for package in apt-transport-https ca-certificates curl software-properties-common; do
        install_package "$package"
    done
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
    sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
    wait_for_apt_lock
    install_package docker-ce
    sudo systemctl enable docker
    sudo systemctl start docker
    sudo usermod -aG docker $SUDO_USER
else
    blue "✅ Docker já está instalado!"
fi

# Instalar Node.js se não estiver instalado
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🟩 Instalando Node.js..."
if ! command -v node &> /dev/null; then
    NODE_VERSION="20.x"
    curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION} | sudo -E bash -
    install_package nodejs

    sudo npm install -g n
else
    blue "✅ Node.js já está instalado!"
fi

# Instalar Python se não estiver instalado
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🐍 Instalando Python..."
for package in python3 python3-pip python3-venv; do
    install_package "$package"
done

# Instalar Java se não estiver instalado
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "☕ Instalando Java..."
if ! dpkg -l | grep -q "openjdk-17-jdk"; then
    install_package openjdk-17-jdk
else
    blue "✅ Java já está instalado!"
fi

# Instalar PostgreSQL se não estiver instalado
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🐘 Instalando PostgreSQL..."
for package in postgresql postgresql-contrib; do
    install_package "$package"
done

# Instalar Redis se não estiver instalado
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🧱 Instalando Redis..."
if ! dpkg -l | grep -q "redis-server"; then
    install_package redis-server
else
    blue "✅ Redis já está instalado!"
fi

# Instalar ferramentas adicionais
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🛠️ Instalando ferramentas adicionais..."
for package in tmux htop; do
    install_package "$package"
done

# Instalar VS Code versão 1.93 se não estiver instalado
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🖥️ Instalando VS Code v1.93..."
if ! command -v code &> /dev/null; then
    wget -qO vscode.deb "https://update.code.visualstudio.com/1.93.0/linux-deb-x64/stable"
    install_package ./vscode.deb
    rm -f vscode.deb

    # Impedir o upgrade do VS Code
    sudo apt-mark hold code
else
    blue "✅ VS Code já está instalado!"
fi

# Garantindo acesso total para extensões de estilização do VS Code
sudo chown -R $SUDO_USER '/usr/share/code/resources/'
sudo chown -R $SUDO_USER '/usr/share/code/resources/app/out'
sudo chmod -R 777 '/usr/share/code/resources/app/out'
sudo chown -R $SUDO_USER '/usr/share/code'
sudo chmod -R 777 '/usr/share/code'

# Instalar Fonts
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔤 Instalando JetBrains Mono Nerd Font..."
if fc-list | grep -qi "JetBrains Mono"; then
    blue "✅ JetBrains Mono já está instalada!"
else
    install_package fonts-jetbrains-mono
fi

progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔤 Instalando Inter Font..."
if fc-list | grep -qi "Inter"; then
    blue "✅ Inter já está instalada!"
else
    install_package fonts-inter
fi

# Instalar Google Chrome se não estiver instalado
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🌎 Instalando Google Chrome..."
if ! dpkg -l | grep -q "google-chrome-stable"; then
    wget -qO chrome.deb "https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb"
    install_package ./chrome.deb
    rm -f chrome.deb
else
    blue "✅ Google Chrome já está instalado!"
fi

# Instalar Discord
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🎮 Instalando Discord..."
if ! command -v discord &> /dev/null; then
    wget -O discord.deb "https://discord.com/api/download?platform=linux&format=deb"
    install_package ./discord.deb
    rm -f discord.deb
else
    blue "✅ Discord já está instalado!"
fi

# Instalar Spotify
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🎵 Instalando Spotify..."
if ! command -v spotify &> /dev/null; then
    if [ ! -f /etc/apt/trusted.gpg.d/spotify.gpg ]; then
        curl -sS https://download.spotify.com/debian/pubkey_C85668DF69375001.gpg | sudo gpg --dearmor --yes -o /etc/apt/trusted.gpg.d/spotify.gpg
        green "✅ Chave GPG do Spotify adicionada!"
    else
        blue "✅ Chave GPG do Spotify já está configurada!"
    fi

    if [ ! -f /etc/apt/sources.list.d/spotify.list ]; then
        echo "deb http://repository.spotify.com stable non-free" | sudo tee /etc/apt/sources.list.d/spotify.list
        green "✅ Repositório do Spotify adicionado!"
    else
        blue "✅ Repositório do Spotify já está configurado!"
    fi

    sudo apt update
    install_package spotify-client
else
    blue "✅ Spotify já está instalado!"
fi

# Instalar WezTerm
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔲 Instalando WezTerm..."
if ! dpkg -l | grep -q "wezterm"; then
    curl -fsSL https://apt.fury.io/wez/gpg.key | sudo gpg --yes --dearmor -o /etc/apt/keyrings/wezterm-fury.gpg
    echo 'deb [signed-by=/etc/apt/keyrings/wezterm-fury.gpg] https://apt.fury.io/wez/ * *' | sudo tee /etc/apt/sources.list.d/wezterm.list
    sudo apt update
    install_package wezterm
else
    blue "✅ WezTerm já está instalado!"
fi

# Configuração do WezTerm
if [ ! -f "$USER_HOME/.wezterm.lua" ]; then
    cp ./wezterm/.wezterm.lua $USER_HOME/.wezterm.lua
    chown $SUDO_USER:$SUDO_USER $USER_HOME/.wezterm.lua
    green "✅ Configuração do WezTerm criada!"
else
    blue "✅ Configuração do WezTerm já existe!"
fi

# Instalar e configurar Oh My Zsh
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "✨ Instalando Oh My Zsh..."

# Instalar Oh My Zsh apenas se ainda não estiver instalado
if [ ! -d "$USER_HOME/.oh-my-zsh" ]; then
    RUNZSH=no sh -c "$(wget -qO- https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"

    # Mover Oh My Zsh para o diretório do usuário correto e configurar o arquivo .zshrc
    if [ "$HOME" != "$USER_HOME" ]; then
        sudo mv "$HOME/.oh-my-zsh" "$USER_HOME/"
    fi

    # Garantir que o usuário tenha permissão sobre os arquivos
    sudo chown -R $SUDO_USER:$SUDO_USER "$USER_HOME/.oh-my-zsh"

    green "✅ Oh My Zsh instalado com sucesso!"

    # # Definir Zsh como shell padrão
    chsh -s $(which zsh) $SUDO_USER
    green "✅ Zsh definido como shell padrão!"
else
    blue "✅ Oh My Zsh já está instalado."
fi

# Instalar zinit
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🚀 Instalando zinit..."
RUNZSH=no sh -c "$(curl --fail --show-error --silent --location https://raw.githubusercontent.com/zdharma-continuum/zinit/HEAD/scripts/install.sh)"

# Instalar Powerlevel10k
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🚀 Instalando Powerlevel10k..."
if [ ! -d "$USER_HOME/.p10k.zsh" ]; then
    git clone --depth=1 https://github.com/romkatv/powerlevel10k.git "${ZSH_CUSTOM:-$USER_HOME/.oh-my-zsh/custom}/themes/powerlevel10k"
    cp ./zsh/.p10k.zsh $USER_HOME/.p10k.zsh
    chown $SUDO_USER:$SUDO_USER $USER_HOME/.p10k.zsh
    green "✅ Powerlevel10k instalado com sucesso!"
else
    blue "✅ Powerlevel10k já está instalado!"
fi

# Instalar CLI tools
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🛠️ Instalando CLI tools..."
for package in eza zoxide; do
    install_package "$package"
done

# Configurar Git
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔧 Configurando Git..."
GIT_USER="EriikGabriel"
GIT_EMAIL="erikgabriel.lins@hotmail.com"

sudo -u "$SUDO_USER" git config --global user.name "$GIT_USER"
sudo -u "$SUDO_USER" git config --global user.email "$GIT_EMAIL"
green "✅ Git configurado com sucesso!"

# Criar chave SSH
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔑 Criando chave SSH..."
SSH_KEY="$USER_HOME/.ssh/id_rsa"
if [ ! -f "$SSH_KEY" ]; then
    ssh-keygen -t rsa -b 4096 -C "$GIT_EMAIL" -f "$SSH_KEY" -N ""
    eval "$(ssh-agent -s)"
    ssh-add "$SSH_KEY"
    green "✅ Chave SSH criada!"
    yellow "🔑 Copie sua chave pública e adicione-a ao GitHub/GitLab:"
    cat "$SSH_KEY.pub"
else
    blue "✅ Chave SSH já existe!"
fi

# Criar diretório de projetos
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "📂 Criando diretórios..."
PROJECTS_DIR="$USER_HOME/www/projects"
UFSCAR_DIR="$USER_HOME/www/ufscar"

if [ ! -d "$PROJECTS_DIR" ]; then
    mkdir -p "$PROJECTS_DIR"
    chown -R $SUDO_USER:$SUDO_USER "$PROJECTS_DIR"
    green "✅ Diretório de projetos criado!"
else
    blue "✅ Diretório de projetos já existe!"
fi

if [ ! -d "$UFSCAR_DIR" ]; then
    mkdir -p "$UFSCAR_DIR"
    chown -R $SUDO_USER:$SUDO_USER "$UFSCAR_DIR"
    green "✅ Diretório da UFSCar criado!"
else
    blue "✅ Diretório da UFSCar já existe!"
fi

# Limpeza
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🧹 Limpando o sistema..."
wait_for_apt_lock
sudo apt autoremove -y
sudo apt clean

if [ "$clear_option" == "y" ]; then
    clear
fi

# Exibir versões dos programas instalados
green "\n🎉 Configuração concluída com sucesso!"

# Exibir mensagem para inserir a chave SSH no GitHub
yellow "\n🔑 Não se esqueça de adicionar sua chave SSH ao GitHub!"
yellow "Copie sua chave pública com o comando:"
blue "cat ~/.ssh/id_rsa.pub"

blue "\n📦 Versões instaladas:"
echo "  🐳 Docker: $(docker --version)"
echo "  🟩 Node.js: $(node -v)"
echo "  📦 NPM: $(npm -v)"
echo "  ✨ Zsh: $(zsh --version)"
echo "  🐍 Python: $(python3 --version)"
echo "  ☕ Java: $(java -version 2>&1 | head -n 1)"
echo "  🐘 PostgreSQL: $(psql --version)"
echo "  🧱 Redis: $(redis-server --version | awk '{print $3}')"
sudo -u $SUDO_USER code --version | head -n 1 | awk '{print "  🖥️  VS Code: "$0}'
echo "  🌎 Google Chrome: $(google-chrome --version)"
echo "  🔲 WezTerm: $(wezterm --version)"
echo "  🔤 JetBrains Mono Nerd Font: Instalado"
echo "  🎵 Spotify: $(spotify --version)"
echo "  🎮 Discord: $(strings $(which discord) | grep -m1 -oP '\d+\.\d+\.\d+')"

yellow "\n🔄 Reinicie o sistema para aplicar todas as alterações."