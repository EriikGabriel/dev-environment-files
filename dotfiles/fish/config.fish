source /usr/share/cachyos-fish-config/cachyos-config.fish

# ==========================================
# Fish configuration
# Converted from zshrc
# ==========================================
if type -q starship
    starship init fish | source
end

# -----------------------
# PATH
# -----------------------

fish_add_path ~/.local/bin

# -----------------------
# FZF
# -----------------------

if type -q fzf
    fzf --fish | source
end

set -gx FZF_DEFAULT_OPTS "\
--color=fg:#CBE0F0,\
bg:#1b1e28,\
hl:#B388FF,\
fg+:#CBE0F0,\
bg+:#303340,\
hl+:#B388FF,\
info:#ADD7FF,\
prompt:#5DE4c7,\
pointer:#5DE4c7,\
marker:#5DE4c7,\
spinner:#5DE4c7,\
header:#5DE4c7"

set -gx FZF_DEFAULT_COMMAND "fd --hidden --strip-cwd-prefix --exclude .git"
set -gx FZF_CTRL_T_COMMAND $FZF_DEFAULT_COMMAND
set -gx FZF_ALT_C_COMMAND "fd --type=d --hidden --strip-cwd-prefix --exclude .git"

# -----------------------
# FZF functions
# -----------------------

function _fzf_compgen_path
    fd --hidden --exclude .git . $argv
end

function _fzf_compgen_dir
    fd --type=d --hidden --exclude .git . $argv
end

# Preview
function __fzf_preview
    if test -d "$argv[1]"
        eza --tree --color=always "$argv[1]" | head -200
    else
        bat -n --color=always --line-range :500 "$argv[1]"
    end
end

set -gx FZF_CTRL_T_OPTS "--preview='__fzf_preview {}'"
set -gx FZF_ALT_C_OPTS "--preview 'eza --tree --color=always {} | head -200'"

source "$__fish_config_dir/themes/vesper.sh"

# -----------------------
# Bat
# -----------------------

set -gx BAT_THEME 1337

# -----------------------
# TheFuck
# -----------------------

if type -q thefuck
    thefuck --alias | source
    thefuck --alias fk | source
end

# -----------------------
# Zoxide
# -----------------------

if type -q zoxide
    zoxide init fish | source
end

# -----------------------
# Environment
# -----------------------

set -gx EDITOR nvim
set -gx VISUAL nvim

# -----------------------
# Custom aliases
# -----------------------

alias c="clear"
alias cd="z"
alias hx="helix"

# -----------------------
# Git shortcuts
# -----------------------

alias gs="git status"
alias ga="git add"
alias gc="git commit"
alias gp="git push"

# ------------------------
# History clear
# ------------------------
function history
    if test (count $argv) -gt 0
        switch $argv[1]
            case clear delete merge save append search
                builtin history $argv
                return
        end
    end

    builtin history --show-time='%F %T ' $argv
end

# bun
set --export BUN_INSTALL "$HOME/.bun"
set --export PATH $BUN_INSTALL/bin $PATH

# opencode
fish_add_path /home/erik/.opencode/bin

# Composio CLI
set --export PATH "$HOME/.local/bin" $PATH
