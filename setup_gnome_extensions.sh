#!/bin/bash

green() { echo -e "\033[32m$1\033[0m"; }
blue() { echo -e "\033[34m$1\033[0m"; }
yellow() { echo -e "\033[33m$1\033[0m"; }
red() { echo -e "\033[31m$1\033[0m"; }

# Verificar se o script está sendo executado com sudo
if [[ $EUID -eq 0 ]]; then
    red "❌ Este script não deve ser executado como root ou com sudo!"
    exit 1
fi

green "🚀 Iniciando setup de extensões do GNOME..."

green "📦 Ativando dark mode..."
gsettings set org.gnome.desktop.interface color-scheme "'prefer-dark'"
green "📦 Ativando tiling nas bordas..."
gsettings set org.gnome.mutter edge-tiling true
green "📦 Reduzindo tamanho dos ícones..."
gsettings set org.gnome.shell.extensions.ding icon-size "small"
green "📦 Ocultando pasta Home no desktop..."
gsettings set org.gnome.shell.extensions.ding show-home false
green "📦 Desabilitando modo painel..."
gsettings set org.gnome.shell.extensions.dash-to-dock extend-height false
green "📦 Ocultando dock automaticamente..."
gsettings set org.gnome.shell.extensions.dash-to-dock dock-fixed false
gsettings set org.gnome.shell.extensions.dash-to-dock autohide true
green "📦 Posicionando dock na parte inferior..."
gsettings set org.gnome.shell.extensions.dash-to-dock dock-position 'BOTTOM'
green "📦 Ajustando tamanho máximo dos ícones..."
gsettings set org.gnome.shell.extensions.dash-to-dock dash-max-icon-size 40
green "📦 Ocultando volumes montados"
gsettings set org.gnome.shell.extensions.dash-to-dock show-mounts false
green "📦 Ocultando redes montadas"
gsettings set org.gnome.shell.extensions.dash-to-dock show-mounts-network false
green "📦 Centralizando novas janelas"
gsettings set org.gnome.mutter center-new-windows true

echo ""

# Função para fixar aplicativos na dock
set_dock_apps() {
    # Recebe a lista de programas como argumentos
    local APPS_LIST=("$@")
    local QUOTED_APPS=()

    # Envolve cada elemento do array em aspas simples
    for APP in "${APPS_LIST[@]}"; do
        local APP_PATH="/usr/share/applications/$APP"

        # Verifica se o arquivo .desktop existe
        if [ -f "$APP_PATH" ]; then
            QUOTED_APPS+=("'$APP'")  # Usa apenas o nome do arquivo .desktop
        else
            red "❌ Arquivo .desktop não encontrado: $APP_PATH"
            return 1
        fi
    done

    # Converte o array de volta para uma lista, separando os elementos por vírgulas
    NEW_APPS=$(IFS=','; echo "[${QUOTED_APPS[*]}]")

    # Define a nova lista de aplicativos fixados
    gsettings set org.gnome.shell favorite-apps "$NEW_APPS"
    green "✅ A dock foi atualizada com os programas na ordem especificada."
}

# Exemplo de uso
green "⚙️ Ajustando programas fixos na dock..."
set_dock_apps "org.gnome.Nautilus.desktop" "org.wezfurlong.wezterm.desktop" "google-chrome.desktop" "code.desktop" "discord.desktop"

# gsettings set org.gnome.shell favorite-apps  "['org.gnome.Nautilus.desktop', 'snap-store_snap-store.desktop', 'yelp.desktop', 'org.wezfurlong.wezterm.desktop', 'google-chrome.desktop', 'code.desktop']"

# Função para instalar uma extensão
install_extension() {
    local extension_id="$1"
    local extension_name="$2"

    # Verificar se a extensão já está instalada
    if gnome-extensions list | grep -q "$extension_id"; then
        green "✅ $extension_name já está instalada."
        return
    fi

    green "📦 Instalando $extension_name..."
    result=$(gdbus call --session \
                        --dest org.gnome.Shell.Extensions \
                        --object-path /org/gnome/Shell/Extensions \
                        --method org.gnome.Shell.Extensions.InstallRemoteExtension \
                        "$extension_id" 2>&1)
    sleep 3 

    # Verificar se a instalação foi bem-sucedida
    if [[ $result == *"successful"* ]]; then
        green "✅ $extension_name instalada com sucesso!"
    else
        red "❌ Falha ao instalar $extension_name. Erro: $result"
        return 1
    fi

    # Esperar até que a extensão esteja instalada e habilitada
    while ! gnome-extensions list | grep -q "$extension_id"; do
        sleep 1
    done
}

# Lista de extensões para instalar
declare -A extensions=(
    ["blur-my-shell@aunetx"]="Blur my Shell"
    ["user-theme@gnome-shell-extensions.gcampax.github.com"]="User Themes"
    ["search-light@icedman.github.com"]="Search Light"
    ["clipboard-indicator@tudmotu.com"]="Clipboard Indicator"
    ["window-title-is-back@fthx"]="Window Title is Back"
    ["compiz-windows-effect@hermes83.github.com"]="Compiz Windows Effect"
    ["compiz-alike-magic-lamp-effect@hermes83.github.com"]="Compiz Alike Magic Lamp Effect"
    ["custom-hot-corners-extended@G-dH.github.com"]="Custom Hot Corners - Extended"
    ["emoji-copy@felipeftn"]="Emoji Copy"
    ["color-picker@tuberry"]="Color Picker"
)

# Instalar cada extensão da lista
for extension_id in "${!extensions[@]}"; do
    install_extension "$extension_id" "${extensions[$extension_id]}" || {
        red "❌ Erro crítico: O script será interrompido."
        exit 1
    }
done

echo ""

green "🛠️ Configurando extensões do GNOME..."

green "⚙️ Configurando extensão Blur my Shell..."
dconf write /org/gnome/shell/extensions/blur-my-shell/dash-to-dock/blur false

green "⚙️ Configurando extensão Search Light..."
dconf write /org/gnome/shell/extensions/search-light/show-panel-icon true
dconf write /org/gnome/shell/extensions/search-light/scale-width 0.28
dconf write /org/gnome/shell/extensions/search-light/scale-height 0.40
dconf write /org/gnome/shell/extensions/search-light/border-thickness 1
dconf write /org/gnome/shell/extensions/search-light/border-radius 7.0
dconf write /org/gnome/shell/extensions/search-light/background-color "(0.0, 0.0, 0.0, 0.6)"

green "⚙️ Configurando extensão Custom Hot Corners - Extended..."
dconf write /org/gnome/shell/extensions/custom-hot-corners-extended/monitor-0-top-right-0/action "'show-desktop-mon'"
dconf write /org/gnome/shell/extensions/custom-hot-corners-extended/misc/panel-menu-enable false

green "⚙️ Configurando extensão Emoji Copy..."
dconf write /org/gnome/shell/extensions/emoji-copy/always-show false
dconf write /org/gnome/shell/extensions/emoji-copy/paste-on-select false

green "⚙️ Configurando extensão Color Picker..."
dconf write /org/gnome/shell/extensions/color-picker/systray-dropper-icon "'tool_color_picker'"

green "⚙️ Configurando extensão User Themes..."
gsettings set org.gnome.desktop.interface gtk-theme "'WhiteSur-Dark-blue'"
dconf write /org/gnome/shell/extensions/user-theme/name "'WhiteSur-Dark-blue'"
gsettings set org.gnome.desktop.interface icon-theme "'WhiteSur-dark'"
gsettings set org.gnome.desktop.interface accent-color "'blue'"

echo "✅ Configuração de extensões do GNOME concluída com sucesso!"
