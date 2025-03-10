#!/bin/bash

# Funções para exibir mensagens coloridas
green() { echo -e "\033[32m$1\033[0m"; }
blue() { echo -e "\033[34m$1\033[0m"; }
yellow() { echo -e "\033[33m$1\033[0m"; }
red() { echo -e "\033[31m$1\033[0m"; }

export -f green blue yellow red

# Verifica se o sudo está disponível
if ! command -v sudo &> /dev/null; then
    red "❌ O comando 'sudo' não está instalado. Instale o sudo e configure-o corretamente."
    exit 1
fi

# Verifica se o usuário tem permissão para usar o sudo
if ! sudo -v; then
    red "❌ Você não tem permissão para usar o sudo. Execute este script como um usuário com permissões sudo."
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

    if [ "$CLEAR_OPT" == "y" ]; then
        clear
    fi

    printf "\r["
    printf "%0.s█" $(seq 1 $completed)   # Parte preenchida
    printf "%0.s░" $(seq 1 $remaining)   # Parte restante
    printf "] %d%% - %s" $((100 * current_step / total_steps)) "$3"
    echo ""
}

export -f wait_for_apt_lock install_package progress_bar

# Caminho do diretório home do usuário original
export USER_HOME=$(eval echo ~$SUDO_USER)
# Nome do usuário original
export USER_NAME=$(basename $USER_HOME)
# Caminho do script de setup
export SCRIPT_DIR=$(dirname "$(realpath "$0")")

# Total de passos (ajustado para o número total de etapas)
export TOTAL_STEPS=29
# Etapa atual (inicializada com 0)
export CURRENT_STEP=0

# Variáveis de configuração globais
export CLEAR_OPT="y"  # Ativar clear do histórico de progresso
export EXEC_TOOLS_SETUP_OPT="y" # Ativar setup de ferramentas
export EXEC_STYLE_SETUP_OPT="y" # Ativar setup de estilização

# Flags controle de fluxo de execução
export SETUP_REBOOT_FLAG="$SCRIPT_DIR/.setup_reboot.flag"

# Desktop files paths
export DESKTOP_SCRIPT_NAME="setup_script.desktop"
export AUTOSTART_FILE="$USER_HOME/.config/autostart/$DESKTOP_SCRIPT_NAME"

# Função para configurar o autostart
setup_autostart() {
    local SCRIPT_PATH=$(realpath "$0")
    local AUTOSTART_DIR="$USER_HOME/.config/autostart"
    local LOCAL_AUTOSTART_FILE="./autostart/$DESKTOP_SCRIPT_NAME"

    # Verificar se o arquivo local existe
    if [[ ! -f "$LOCAL_AUTOSTART_FILE" ]]; then
        red "❌ Arquivo $LOCAL_AUTOSTART_FILE não encontrado!"
        return 1
    fi

    # Criar diretório autostart se não existir
    mkdir -p "$AUTOSTART_DIR"

    # Copiar o arquivo .desktop para o diretório de autostart
    cp "$LOCAL_AUTOSTART_FILE" "$AUTOSTART_DIR/"

    # Atualizar o caminho do script no arquivo .desktop
    sed -i "s|{SCRIPT_PATH}|$SCRIPT_PATH|g" "$AUTOSTART_DIR/$DESKTOP_SCRIPT_NAME"

    green "✅ Arquivo .desktop configurado para executar o script após o reboot."
}

# Função para remover o autostart
remove_autostart() {
    if [ -f "$AUTOSTART_FILE" ]; then
        rm -f "$AUTOSTART_FILE"
        green "✅ Autostart removido."
    else
        green "✅ Nenhum arquivo de autostart encontrado."
    fi
}

# Função para exibir o menu interativo
show_menu() {
    while true; do
        clear
        green "🚀 Menu de Setup (main) - Made by EriikGabriel"
        echo ""
        blue "1. Ativar/Desativar clear do histórico de progresso (Atual: $CLEAR_OPT)"
        blue "2. Ativar/Desativar setup de ferramentas (Atual: $EXEC_TOOLS_SETUP_OPT)"
        blue "3. Ativar/Desativar setup de estilização (Atual: $EXEC_STYLE_SETUP_OPT)"
        blue "4. Iniciar setup"
        blue "5. Sair"
        echo ""
        read -p "Escolha uma opção: " MENU_OPT

        case $MENU_OPT in
            1)
                if [ "$CLEAR_OPT" == "y" ]; then
                    CLEAR_OPT="n"
                else
                    CLEAR_OPT="y"
                fi
                green "✅ Clear do histórico de progresso definido como: $CLEAR_OPT"
                sleep 1
                ;;
            2)
                if [ "$EXEC_TOOLS_SETUP_OPT" == "y" ]; then
                    EXEC_TOOLS_SETUP_OPT="n"
                else
                    EXEC_TOOLS_SETUP_OPT="y"
                fi
                green "✅ Ativação de setup de ferramentas definido como: $EXEC_TOOLS_SETUP_OPT"
                sleep 1
                ;;
            3)
                if [ "$EXEC_STYLE_SETUP_OPT" == "y" ]; then
                    EXEC_STYLE_SETUP_OPT="n"
                else
                    EXEC_STYLE_SETUP_OPT="y"
                fi
                green "✅ Ativação de setup de estilização definido como: $EXEC_STYLE_SETUP_OPT"
                sleep 1
                ;;
            4)
                green "✅ Iniciando setup..."
                sleep 1
                break
                ;;
            5)
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
else 
    source "$SETUP_REBOOT_FLAG"
fi

if [ "$EXEC_TOOLS_SETUP_OPT" == "y" ]; then
    # Executar script de setup de ferramentas
    yellow "🔧 Executando setup de ferramentas..."
    source $SCRIPT_DIR/setup_tools.sh
fi

if [ "$EXEC_STYLE_SETUP_OPT" == "y" ]; then
    # Executar script de setup de estilização
    yellow "🎨 Executando setup de estilização..."
    source $SCRIPT_DIR/setup_style.sh
fi

# Exibir versões dos programas instalados
green "🎉 Ambiente de desenvolvimento configurado com sucesso!"

# Exibir mensagem para inserir a chave SSH no GitHub
if [ "$EXEC_TOOLS_SETUP_OPT" == "y" ]; then
    yellow "\n🔑 Não se esqueça de adicionar sua chave SSH ao GitHub!"
    blue "cat ~/.ssh/id_ed25519.pub"
fi

blue "\n📦 Versões instaladas:"
if [ "$EXEC_TOOLS_SETUP_OPT" == "y" ]; then
    echo "  🐳 Docker: $(docker --version)"
    echo "  🟩 Node.js: $(node -v)"
    echo "  📦 NPM: $(npm -v)"
    echo "  ✨ Zsh: $(zsh --version)"
    echo "  🐍 Python: $(python3 --version)"
    echo "  ☕ Java: $(java -version 2>&1 | head -n 1)"
    echo "  🐘 PostgreSQL: $(psql --version)"
    echo "  🧱 Redis: $(redis-server --version | awk '{print $3}')"
    code --version | head -n 1 | awk '{print "  🖥️  VS Code: "$0}'
    echo "  🌎 Google Chrome: $(google-chrome --version)"
    echo "  🔲 WezTerm: $(wezterm --version)"
    echo "  🔤 JetBrains/Inter Font: Instalado"
    if [ "$INSTALL_ENTERTAINMENT_OPT" == "y" ]; then
        echo "$(spotify --version)" | sed -n 's/.*version \([^,]*\).*/  🎵 Spotify: \1/p'
        echo "  🎮 Discord: $(strings $(which discord) | grep -m1 -oP '\d+\.\d+\.\d+')"
    fi
fi
if [ "$EXEC_STYLE_SETUP_OPT" == "y" ]; then
    echo "  🎨 Tema GTK: $(gsettings get org.gnome.desktop.interface gtk-theme)"
fi

echo ""

# Verificar se o script já foi executado após a reinicialização
if [ -f "$SETUP_REBOOT_FLAG" ]; then
    # Remover o autostart após a execução
    remove_autostart

    rm -f "$SETUP_REBOOT_FLAG"

    green "\n✅ Setup finalizado e concluído com sucesso!"
else
    # Configurar o autostart
    setup_autostart

    touch $SETUP_REBOOT_FLAG
    echo "CLEAR_OPT=$CLEAR_OPT" > "$SETUP_REBOOT_FLAG"

    # Reiniciar o sistema com contagem regressiva
    yellow "🔧 Reiniciando o sistema para aplicar as alterações..."
    yellow "🔧 O script será iniciado automaticamente após a reinicialização em um terminal gráfico!"
    yellow "🕒 O sistema será reiniciado em:"

    for i in {10..1}; do
        echo -ne "⏳ $i segundos...\r"
        sleep 1
    done

    echo -ne "🚀 Reiniciando o sistema agora!            \r"
    sleep 1
    reboot
fi
