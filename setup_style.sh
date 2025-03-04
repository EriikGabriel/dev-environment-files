#!/bin/bash

export CUSTOM_WALLPAPER_OPT="y"  # Instalar wallpaper customizado

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

blue "\n📦 Instalando pacotes de estilização..."

# Instalar ferramentas essenciais
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🧰 Instalando ferramentas essenciais..."
for package in sassc gnome-tweaks gnome-shell-extension-manager meson gettext pkg-config make; do
    install_package "$package"
done

# Clonar e instalar Dash to Dock
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🚢 Instalando Dash to Dock..."
if [ ! -d "dash-to-dock" ]; then
    git clone https://github.com/micheleg/dash-to-dock.git
fi
cd dash-to-dock
meson setup build --prefix=/usr
ninja -C build
sudo ninja -C build install
cd ..

# Reiniciar o GNOME Shell para aplicar a extensão
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🔄 Reiniciando o GNOME Shell..."
gnome-shell --replace &



# Habilitar algumas configurações do GNOME Desktop
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🖥️ Configurando o GNOME desktop..."
gsettings set org.gnome.mutter edge-tiling true # Ativar tiling nas bordas
gsettings set org.gnome.shell.extensions.ding icon-size "small" # Reduzir tamanho dos ícones
gsettings set org.gnome.shell.extensions.ding icon-volumes false # Ocultar ícones de volumes
gsettings set org.gnome.shell.extensions.ding show-home false # Ocultar pasta Home no desktop

# Configurar Dash to Dock
gsettings set org.gnome.shell.extensions.dash-to-dock dock-fixed false # Desabilitar Panel Mode
gsettings set org.gnome.shell.extensions.dash-to-dock autohide true # Ocultar automaticamente
gsettings set org.gnome.shell.extensions.dash-to-dock dock-position 'BOTTOM' # Posicionar no fundo
gsettings set org.gnome.shell.extensions.dash-to-dock dash-max-icon-size 40 # Tamanho dos ícones


# Instalar e configurar flatpak e flathub
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "📦 Instalando Flatpak e Flathub..."
if ! command -v flatpak &> /dev/null; then
    install_package flatpak
    flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo
fi

# Instalar tema GTK
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🎨 Instalando tema GTK..."
# install_package materia-gtk-theme



# Limpeza
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🧹 Limpando o sistema..."
wait_for_apt_lock
sudo apt autoremove -y
sudo apt clean

if [ "$CLEAR_OPT" == "y" ]; then
    clear
fi