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

# Limpeza
progress_bar $TOTAL_STEPS $((++CURRENT_STEP)) "🧹 Limpando o sistema..."
wait_for_apt_lock
sudo apt autoremove -y
sudo apt clean

if [ "$CLEAR_OPT" == "y" ]; then
    clear
fi