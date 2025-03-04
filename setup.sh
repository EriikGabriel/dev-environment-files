#!/bin/bash

# Funções para exibir mensagens coloridas
green() { echo -e "\033[32m$1\033[0m"; }
blue() { echo -e "\033[34m$1\033[0m"; }
yellow() { echo -e "\033[33m$1\033[0m"; }
red() { echo -e "\033[31m$1\033[0m"; }

export -f green blue yellow red

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

# Endereço do diretório home do usuário original
export USER_HOME=$(eval echo ~$SUDO_USER)
# Nome do usuário original
export USER_NAME=$(basename $USER_HOME)

# Total de passos (ajustado para o número total de etapas)
export TOTAL_STEPS=29
# Etapa atual (inicializada com 0)
export CURRENT_STEP=0

# Variáveis de configuração globais
export CLEAR_OPT="y"  # Ativar clear do histórico de progresso
export EXEC_TOOLS_SETUP_OPT="y" # Ativar setup de ferramentas
export EXEC_STYLE_SETUP_OPT="y" # Ativar setup de estilização

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
show_menu

if [ $EXEC_TOOLS_SETUP_OPT == "y" ]; then
    # Executar script de setup de ferramentas
    yellow "🔧 Executando setup de ferramentas..."
    source setup_tools.sh
fi

if [ $EXEC_STYLE_SETUP_OPT == "y" ]; then
    # Executar script de setup de estilização
    yellow "🎨 Executando setup de estilização..."
    source setup_style.sh
fi

# Exibir versões dos programas instalados
green "🎉 Ambiente de desenvolvimento configurado com sucesso!"

# Exibir mensagem para inserir a chave SSH no GitHub
yellow "\n🔑 Não se esqueça de adicionar sua chave SSH ao GitHub!"
blue "cat ~/.ssh/id_ed25519.pub"

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
echo "  🔤 JetBrains/Inter Font: Instalado"
if [ "$INSTALL_ENTERTAINMENT_OPT" == "y" ]; then
    echo "$(spotify --version)" | sed -n 's/.*version \([^,]*\).*/  🎵 Spotify: \1/p'
    echo "  🎮 Discord: $(strings $(which discord) | grep -m1 -oP '\d+\.\d+\.\d+')"
fi
if [ "$EXEC_STYLE_SETUP_OPT" == "y" ]; then
    echo "  🎨 Tema GTK: $(gsettings get org.gnome.desktop.interface gtk-theme)"
fi

yellow "\n🔄 Reinicie o sistema para aplicar todas as alterações."
