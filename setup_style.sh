#!/bin/bash

export CUSTOM_WALLPAPER_OPT="y"  # Instalar wallpaper customizado

# Funções para cores no terminal
green() { echo -e "\033[32m$1\033[0m"; }
blue() { echo -e "\033[34m$1\033[0m"; }
yellow() { echo -e "\033[33m$1\033[0m"; }
red() { echo -e "\033[31m$1\033[0m"; }

if [ $EXEC_TOOLS_SETUP_OPT == "y" && $EXEC_STYLE_SETUP_OPT == "y"]; then
    TOTAL_STEPS=TOTAL_STEPS+5
else
    TOTAL_STEPS=5
    CURRENT_STEP=0
fi

# Função para exibir o menu interativo
show_menu() {
    while true; do
        clear
        green "🚀 Menu de Configuração (style)"
        echo ""
        blue "1. Ativar/Desativar wallpaper customizado (Atual: $CUSTOM_WALLPAPER_OPT)"
        blue "2. Iniciar instalação"
        blue "3. Sair"
        echo ""
        read -p "Escolha uma opção: " MENU_OPT

        case $MENU_OPT in
            1)
                if [ "$CUSTOM_WALLPAPER_OPT" == "y" ]; then
                    CUSTOM_WALLPAPER_OPT="n"
                else
                    CUSTOM_WALLPAPER_OPT="y"
                fi
                green "✅ Instalação de utilitários de entretenimento definida como: $CUSTOM_WALLPAPER_OPT"
                sleep 1
                ;;
            2)
                green "✅ Iniciando instalação..."
                sleep 1
                break
                ;;
            3)
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

yellow "🚀 Iniciando configuração..."

# Atualizar sistema
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🛠️ Atualizando sistema..."
wait_for_apt_lock
sudo apt update && sudo apt upgrade -y && sudo apt dist-upgrade -y
green "\n✅ Sistema atualizado!"

# Instalar ferramentas essenciais
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🧰 Instalando ferramentas essenciais..."
for package in sassc dbus-x11 gnome-tweaks gnome-shell-extension-manager meson gettext make; do
    install_package "$package"
done

export $(dbus-launch)

# Variáveis de configuração do GNOME
GNOME_DOCK_AUTOHIDE=$(sudo -u $USER_NAME gsettings get org.gnome.shell.extensions.dash-to-dock autohide)

if [[ "$GNOME_DOCK_AUTOHIDE" == true ]]; then
    green "✅ Extensões do GNOME já foram instaladas."
else
    # Instalar e configurar extensões do GNOME
    progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🧩 Instalando extensões do GNOME..."

    yellow "⚠️ Está etapa requer intervenção manual, rode o script setup_gnome_extensions.sh (mas mantenha este em execução) -> ./setup_gnome_extensions.sh..."

    # Aguardar até que o arquivo de controle seja criado
    while [ ! -f /tmp/setup_gnome_extensions_done ]; do
        sleep 1 # Aguardar 1 segundo antes de verificar novamente
    done

    # Remover arquivo de controle
    rm -rf /tmp/setup_gnome_extensions_done
    green "✅ Script de extensões do GNOME concluído. Continuando a configuração..."
fi

# Instalar e configurar flatpak e flathub
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "📦 Instalando Flatpak e Flathub..."
if ! command -v flatpak &> /dev/null; then
    sudo apt install -y flatpak
    flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo
fi

# Instalar tema GTK WhiteSur
if [ ! -d "/usr/share/themes/WhiteSur-Dark" ]; then
    progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🎨 Instalando tema WhiteSur..."
    git clone https://github.com/vinceliuice/WhiteSur-gtk-theme.git --depth=1
    cd WhiteSur-gtk-theme

    # Instalando tema WhiteSur
    ./install.sh -n WhiteSur -t all -m -N glassy -l --shell -i ubuntu -h smaller --round --silent-mode

    # Instalando tweaks do WhiteSur
    sudo ./tweaks.sh -g -i ubuntu -F -d --silent-mode

    sudo flatpak override --filesystem=xdg-config/gtk-3.0 && sudo flatpak override --filesystem=xdg-config/gtk-4.0

    cd ..
    rm -rf WhiteSur-gtk-theme
    green "✅ Tema WhiteSur instalado com sucesso!"
else
    green "✅ Tema WhiteSur já está instalado."
fi

# Instalar ícones WhiteSur
if [ ! -d "$USER_HOME/.local/share/icons/WhiteSur" ]; then
    progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🎨 Instalando ícones WhiteSur..."
    git clone https://github.com/vinceliuice/WhiteSur-icon-theme.git --depth=1
    cd WhiteSur-icon-theme

    # Instalando ícones WhiteSur
    ./install.sh -d "$USER_HOME/.local/share/icons" -n WhiteSur -a

    cd ..
    rm -rf WhiteSur-icon-theme
    green "✅ Ícones WhiteSur instalados com sucesso!"
else
    green "✅ Ícones WhiteSur já estão instalados."
fi

# Habilitar GTK e ícones
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🎨 Habilitando tema e ícones..."
CURRENT_GTK_THEME=$(gsettings get org.gnome.desktop.interface gtk-theme)
if [ CURRENT_GTK_THEME != "'WhiteSur-Dark'" ]; then
    sudo -u $USER_NAME gsettings set org.gnome.desktop.interface gtk-theme "WhiteSur-Dark"
    sudo -u $USER_NAME dconf write /org/gnome/shell/extensions/user-theme/name "'WhiteSur-Dark'"
    sudo -u $USER_NAME gsettings set org.gnome.desktop.interface icon-theme "WhiteSur-dark"

    TERMINAL_DESKTOP_FILE="/usr/share/applications/org.wezfurlong.wezterm.desktop"
    TERMINAL_NEW_ICON="$USER_HOME/.local/share/icons/WhiteSur/apps/scalable/org.gnome.Terminal.svg"
    chmod +w "$TERMINAL_DESKTOP_FILE"
    sudo sed -i "s|^Icon=.*|Icon=$TERMINAL_NEW_ICON|" "$TERMINAL_DESKTOP_FILE"
    sudo update-icon-caches /home/erikg/.local/share/icons/WhiteSur

    green "✅ Tema e ícones WhiteSur habilitados com sucesso!"
else
    green "✅ Tema e ícones WhiteSur já estão habilitados."
fi

# Limpeza
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🧹 Limpando o sistema..."
sudo apt autoremove -y
sudo apt clean

green "✅ Configuração concluída com sucesso!"