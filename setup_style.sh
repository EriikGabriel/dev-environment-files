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
if [ ! -f "$AUTOSTART_FILE" ]; then
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
for package in sassc dbus-x11 gnome-tweaks gnome-shell-extension-manager meson gettext make; do
    install_package "$package"
done

# Variáveis de configuração do GNOME
GNOME_DOCK_AUTOHIDE=$(sudo -u $USER_NAME gsettings get org.gnome.shell.extensions.dash-to-dock autohide)

if [[ "$GNOME_DOCK_AUTOHIDE" == false ]] || [ -f "$AUTOSTART_FILE" ]; then
    # Instalar e configurar extensões do GNOME
    progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🧩 Instalando extensões do GNOME..."
    source $SCRIPT_DIR/setup_gnome_extensions.sh

else
    blue "✅ Extensões do GNOME já foram instaladas."
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
    sudo ./install.sh -n WhiteSur -t all -m -N glassy -l --shell -i ubuntu -h smaller --round --silent-mode

    # Instalando tweaks do WhiteSur
    sudo ./tweaks.sh -g -i ubuntu -F -d --silent-mode

    sudo flatpak override --filesystem=xdg-config/gtk-3.0 && sudo flatpak override --filesystem=xdg-config/gtk-4.0

    cd ..
    rm -rf WhiteSur-gtk-theme
    green "✅ Tema WhiteSur instalado com sucesso!"
else
    blue "✅ Tema WhiteSur já está instalado."
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
    blue "✅ Ícones WhiteSur já estão instalados."
fi

# Habilitar GTK e ícones
CURRENT_GTK_THEME=$(gsettings get org.gnome.desktop.interface gtk-theme)
if [ "$CURRENT_GTK_THEME" != "'WhiteSur-Dark-blue'" ] || [ -f "$AUTOSTART_FILE" ]; then
    progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🎨 Habilitando tema e ícones..."
    gsettings set org.gnome.desktop.interface gtk-theme "'WhiteSur-Dark-blue'"
    dconf write /org/gnome/shell/extensions/user-theme/name "'WhiteSur-Dark-blue'"
    gsettings set org.gnome.desktop.interface icon-theme "'WhiteSur-dark'"
    gsettings set org.gnome.desktop.interface accent-color "'blue'"

    TERMINAL_DESKTOP_FILE="/usr/share/applications/org.wezfurlong.wezterm.desktop"
    TERMINAL_NEW_ICON="$USER_HOME/.local/share/icons/WhiteSur/apps/scalable/org.gnome.Terminal.svg"
    sudo chmod +w "$TERMINAL_DESKTOP_FILE"
    sudo sed -i "s|^Icon=.*|Icon=$TERMINAL_NEW_ICON|" "$TERMINAL_DESKTOP_FILE"
    sudo update-icon-caches $USER_HOME/.local/share/icons/WhiteSur

    green "✅ Tema e ícones WhiteSur habilitados com sucesso!"
else
    blue "✅ Tema e ícones WhiteSur já estão habilitados."
fi

WALLPAPER_URL="https://www.iclarified.com/images/news/91914/440181/440181.jpg"
WALLPAPER_PATH="/usr/share/backgrounds/mac-wallpaper.jpg"
CURRENT_WALLPAPER=$(gsettings get org.gnome.desktop.background picture-uri-dark)
if [ "$CUSTOM_WALLPAPER_OPT" == "y" ]; then
    if [ "$CURRENT_WALLPAPER" != "'file://$WALLPAPER_PATH'" ]; then
        # Instalar wallpaper customizado
        progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🖼️ Instalando wallpaper customizado..."
        # Baixar o wallpaper
        echo "⬇️ Baixando wallpaper..."
        sudo wget -O "$WALLPAPER_PATH" "$WALLPAPER_URL"
        sudo chmod 644 "$WALLPAPER_PATH"

        # Verificar se o arquivo foi baixado corretamente
        if [ ! -f "$WALLPAPER_PATH" ]; then
            red "❌ Erro: O wallpaper não foi baixado corretamente."
            exit 1
        fi

        echo "🎨 Alterando wallpaper..."
        gsettings set org.gnome.desktop.background picture-uri "'file://$WALLPAPER_PATH'"
        gsettings set org.gnome.desktop.background picture-uri-dark "'file://$WALLPAPER_PATH'"
        green "✅ Wallpaper customizado instalado com sucesso!"
    else
        blue "✅ Wallpaper customizado já está instalado."
    fi
fi

# Limpeza
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🧹 Limpando o sistema..."
sudo apt autoremove -y
sudo apt clean

green "✅ Configuração concluída com sucesso!"