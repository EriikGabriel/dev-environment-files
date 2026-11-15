#!/bin/bash

export INSTALL_ENTERTAINMENT_OPT="y"  # Instalar utilitários de entretenimento
export VSCODE_VERSION_OPT="latest"  # Versão do VS Code (1.93 ou latest)

# Função para exibir o menu interativo
show_menu() {
    while true; do
        clear
        green "🚀 Menu de Configuração (tools)"
        echo ""
        blue "1. Ativar/Desativar instalação de utilitários de entretenimento (Atual: $INSTALL_ENTERTAINMENT_OPT)"
        blue "2. Escolher versão do VS Code (Atual: $VSCODE_VERSION_OPT)"
        blue "3. Iniciar instalação"
        blue "4. Sair"
        echo ""
        read -p "Escolha uma opção: " MENU_OPT

        case $MENU_OPT in
            1)
                if [ "$INSTALL_ENTERTAINMENT_OPT" == "y" ]; then
                    INSTALL_ENTERTAINMENT_OPT="n"
                else
                    INSTALL_ENTERTAINMENT_OPT="y"
                fi
                green "✅ Instalação de utilitários de entretenimento definida como: $INSTALL_ENTERTAINMENT_OPT"
                sleep 1
                ;;
            2)
                echo ""
                blue "Escolha a versão do VS Code:"
                blue "1. Versão mais recente"
                blue "2. Versão 1.93 (específica)"
                read -p "Opção: " vscode_choice
                if [ "$vscode_choice" == "2" ]; then
                    VSCODE_VERSION_OPT="1.93"
                else
                    VSCODE_VERSION_OPT="latest"
                fi
                green "✅ Versão do VS Code definida como: $VSCODE_VERSION_OPT"
                sleep 1
                ;;
            3)
                green "✅ Iniciando instalação..."
                sleep 1
                break
                ;;
            4)
                red "❌ Saindo do script..."
                exit 0
                ;;
            *)
                red "❌ Opção inválida!"
                sleep 1
                ;;
        esac
    done
}

# Exibir o menu interativo
if [ ! -f "$SETUP_REBOOT_FLAG" ]; then
    show_menu
fi

yellow "🚀 Iniciando configuração..."

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
if [ ! -f "$USER_HOME/.zshrc" ] || [ -f "$SETUP_REBOOT_FLAG" ]; then
    cp $SCRIPT_DIR/../dotfiles/zsh/.zshrc $USER_HOME/.zshrc
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

    # Corrige armazenamento de chave GPG do docker em chaveiro legado
    sudo apt-key list | grep -B 1 "docker" | grep "pub" | awk '{print $2}' | sudo apt-key del
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo tee /etc/apt/trusted.gpg.d/docker.asc > /dev/null
    sudo apt update
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

# Instalar VS Code
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🖥️ Instalando VS Code..."
if ! command -v code &> /dev/null; then
    if [ "$VSCODE_VERSION_OPT" == "1.93" ]; then
        wget -qO vscode.deb "https://update.code.visualstudio.com/1.93.0/linux-deb-x64/stable"
        install_package ./vscode.deb
        rm -f vscode.deb
    else
        wget -qO vscode.deb "https://code.visualstudio.com/sha/download?build=stable&os=linux-deb-x64"
        install_package ./vscode.deb
        rm -f vscode.deb
    fi

    # Impedir o upgrade do VS Code
    sudo apt-mark hold code
else
    blue "✅ VS Code já está instalado!"
fi

# Instalar a CLI do GitHub (gh)
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔐 Instalando a CLI do GitHub..."
if ! command -v gh &> /dev/null; then
    curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
    install_package gh
    blue "✅ CLI do GitHub instalada com sucesso!"
else
    blue "✅ CLI do GitHub já está instalada!"
fi

# Autenticar com o GitHub
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔑 Autenticando com o GitHub..."
if ! gh auth status &> /dev/null; then
    blue "🔑 Por favor, siga as instruções no navegador para autenticar com o GitHub..."
    gh auth login
else
    blue "✅ Autenticação com o GitHub já está configurada!"
fi

# Recuperar o token de acesso do GitHub
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔑 Recuperando token de acesso do GitHub..."
SYNC_TOKEN=$(gh auth token)
if [ -z "$SYNC_TOKEN" ]; then
    red "❌ Erro: Não foi possível recuperar o token do GitHub."
    exit 1
else
    blue "✅ Token do GitHub recuperado com sucesso!"
fi

# Configurar o Settings Sync no VS Code
SYNC_STATUS=$(code --list-extensions | grep "github.copilot" || true)

if [ -z "$SYNC_STATUS" ]; then
    progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "⚙️ Configurando Settings Sync no VS Code..."

    if command -v code &> /dev/null; then
        code --sync on --github-token $SYNC_TOKEN
        blue "✅ Settings Sync configurado com sucesso!"
    else
        red "❌ Erro: VS Code não está instalado. Não foi possível configurar o Settings Sync."
    fi
else
    blue "✅ Settings Sync já está configurado!"
fi

# Garantindo acesso total para extensões de estilização do VS Code
sudo chown -R $USER_NAME '/usr/share/code/resources/'
sudo chown -R $USER_NAME '/usr/share/code/resources/app/out'
sudo chmod -R 777 '/usr/share/code/resources/app/out'
sudo chown -R $USER_NAME '/usr/share/code'
sudo chmod -R 777 '/usr/share/code'

# Instalar Fonts
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔤 Instalando JetBrains Mono Nerd Font..."
if fc-list | grep -qi "JetBrains Mono"; then
    blue "✅ JetBrains Mono já está instalada!"
else
    install_package fonts-jetbrains-mono

    wget -P $USER_HOME/.local/share/fonts https://github.com/ryanoasis/nerd-fonts/releases/download/v3.0.2/JetBrainsMono.zip \
    && cd $USER_HOME/.local/share/fonts \
    && unzip JetBrainsMono.zip \
    && rm JetBrainsMono.zip \
    && fc-cache -fv
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

# Instalar Discord (se habilitado)
if [ "$INSTALL_ENTERTAINMENT_OPT" == "y" ]; then
    progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🎮 Instalando Discord..."
    if ! command -v discord &> /dev/null; then
        wget -O discord.deb "https://discord.com/api/download?platform=linux&format=deb"
        install_package ./discord.deb
        rm -f discord.deb
    else
        blue "✅ Discord já está instalado!"
    fi
fi

# Instalar Spotify (se habilitado)
if [ "$INSTALL_ENTERTAINMENT_OPT" == "y" ]; then
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
    cp $SCRIPT_DIR/../dotfiles/wezterm/.wezterm.lua $USER_HOME/.wezterm.lua
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
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🚀 Instalando Zinit..."
if [ ! -d "$USER_HOME/.local/share/zinit/zinit.git" ]; then
   RUNZSH=no sh -c "$(curl --fail --show-error --silent --location https://raw.githubusercontent.com/zdharma-continuum/zinit/HEAD/scripts/install.sh)"
    green "✅ Zinit instalado com sucesso!"
else
    blue "✅ Zinit já está instalado!"
fi

# Instalar Powerlevel10k
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🚀 Instalando Powerlevel10k..."
if [ ! -f "$USER_HOME/.p10k.zsh" ]; then
    git clone --depth=1 https://github.com/romkatv/powerlevel10k.git "${ZSH_CUSTOM:-$USER_HOME/.oh-my-zsh/custom}/themes/powerlevel10k"
    cp $SCRIPT_DIR/../dotfiles/zsh/.p10k.zsh $USER_HOME/.p10k.zsh
    chown $SUDO_USER:$SUDO_USER $USER_HOME/.p10k.zsh
    green "✅ Powerlevel10k instalado com sucesso!"
else
    blue "✅ Powerlevel10k já está instalado!"
fi

# Instalar CLI tools
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🛠️ Instalando CLI tools..."
for package in fastfetch eza zoxide fd-find bat git-delta thefuck; do
    install_package "$package"
done
if ! command -v fzf &> /dev/null; then
    sudo curl -fsSL https://github.com/junegunn/fzf/releases/download/v0.60.3/fzf-0.60.3-linux_amd64.tar.gz | sudo tar -xz -C /usr/local/bin/
    sudo ln -sf /usr/local/bin/fzf /usr/local/bin/fzf-tmux
    green "✅ fzf instalado com sucesso!"
else
    blue "✅ fzf já está instalado!"
fi

# Links simbólicos para ferramentas CLI
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔗 Criando links simbólicos..."
mkdir -p $USER_HOME/.local/bin
for cmd in /usr/bin/batcat /usr/bin/delta /usr/bin/fdfind /usr/bin/fzf; do
    target="$USER_HOME/.local/bin/$(basename $cmd)"
    [ ! -e $target ] && ln -sf $cmd $target
done
green "✅ Links simbólicos criados!"

# Configurar fastfetch
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔧 Configurando fastfetch..."
FASTFETCH_CONFIG_FILE="$USER_HOME/.config/fastfetch/config.jsonc"
if ! grep -q "cli-ascii-1.txt" "$FASTFETCH_CONFIG_FILE"; then
    # Gerar arquivo de configuração
    sudo -u $USER_NAME fastfetch --gen-config-force

    # Criar diretório de logos e copiar arquivo
    sudo mkdir -p $USER_HOME/.config/fastfetch/logos

    # Copiar todos os logos
    sudo cp -r $SCRIPT_DIR/../dotfiles/fastfetch/logos/* $USER_HOME/.config/fastfetch/logos

    # Substituir arquivo de configuração
    CONFIG_FILE_NAME="arch"
    sudo cp $SCRIPT_DIR/../dotfiles/fastfetch/$CONFIG_FILE_NAME.jsonc $FASTFETCH_CONFIG_FILE
    green "✅ fastfetch configurado com sucesso!"
else
    blue "✅ Configuração do fastfetch já existe!"
fi

# Configurar bat
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔧 Configurando bat..."
if [ ! -d "$USER_HOME/.config/bat/themes" ]; then
    mkdir -p "$USER_HOME/.config/bat/themes"
    curl -L -o "$USER_HOME/.config/bat/themes/pmndrs.tmTheme" "http://raw.githubusercontent.com/drcmda/poimandres-theme/refs/heads/main/pmndrs.tmTheme"
    batcat cache --build
    green "✅ bat configurado com sucesso!"
else
    blue "✅ Configuração do bat já existe!"
fi

# Configurar Git
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔧 Configurando Git..."
if ! git config user.name &> /dev/null || ! git config user.email &> /dev/null; then
    read -p "Digite seu nome: " GIT_USER
    read -p "Digite seu e-mail: " GIT_EMAIL

    sudo -u "$USER_NAME" git config --global user.name "$GIT_USER"
    sudo -u "$USER_NAME" git config --global user.email "$GIT_EMAIL"

    green "✅ Git configurado com sucesso!"
else
    blue "✅ Git já está configurado!"
    blue "  Nome: $(git config user.name)"
    blue "  E-mail: $(git config user.email)"
fi

# Criar chave SSH
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔑 Criando chave SSH..."
SSH_KEY="$USER_HOME/.ssh/id_ed25519"
if [ ! -f "$SSH_KEY" ]; then
    ssh-keygen -t ed25519 -C "$GIT_EMAIL"
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

if [ "$CLEAR_OPT" == "y" ]; then
    clear
fi
