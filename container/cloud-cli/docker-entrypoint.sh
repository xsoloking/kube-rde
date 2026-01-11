#!/bin/bash
set -e

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Ensure we are running as claudeui
if [ "$(id -u)" = "0" ]; then
    echo -e "${YELLOW}Running as root, fixing permissions...${NC}"
    
    # Fix permissions for home directory (important for PVCs)
    chown -R claudeui:claudeui /home/claudeui
    
    # Also ensure the workspace directory specifically if it's a mount point
    if [ -d "/home/claudeui/workspace" ]; then
        chown claudeui:claudeui /home/claudeui/workspace
    fi

    echo -e "${GREEN}Dropping privileges to claudeui...${NC}"
    exec gosu claudeui "$0" "$@"
fi

echo -e "${GREEN}=== Initializing Claude Code UI ===${NC}"

# 1. 创建必要的目录结构（幂等操作）
echo -e "${YELLOW}Setting up directory structure...${NC}"
mkdir -p \
    ${HOME}/.claude-code-ui/data \
    ${HOME}/.claude-code-ui/logs \
    ${HOME}/.claude-code-ui/cache \
    ${HOME}/.claude-code-ui/tmp \
    ${HOME}/.claude \
    ${HOME}/.cursor \
    ${HOME}/.ssh \
    ${HOME}/.config \
    ${HOME}/.local/share \
    ${HOME}/go/src \
    ${HOME}/go/bin \
    ${HOME}/go/pkg \
    ${HOME}/workspace

# 2. 设置 SSH 配置（如果不存在）
if [ ! -f ${HOME}/.ssh/config ]; then
    echo -e "${YELLOW}Creating SSH config...${NC}"
    cat > ${HOME}/.ssh/config <<EOF
Host *
    StrictHostKeyChecking no
    UserKnownHostsFile=/dev/null
    ServerAliveInterval 60
    ServerAliveCountMax 3
EOF
    chmod 600 ${HOME}/.ssh/config
fi

# 设置 SSH 权限
chmod 700 ${HOME}/.ssh 2>/dev/null || true
find ${HOME}/.ssh -type f -exec chmod 600 {} \; 2>/dev/null || true

# 3. 配置 Git（幂等操作）
echo -e "${YELLOW}Configuring Git...${NC}"
git config --global user.name "${GIT_USER_NAME:-Claude Code UI}" 2>/dev/null || true
git config --global user.email "${GIT_USER_EMAIL:-claude@example.com}" 2>/dev/null || true
git config --global init.defaultBranch main 2>/dev/null || true
git config --global core.editor "vim" 2>/dev/null || true
git config --global pull.rebase false 2>/dev/null || true
git config --global credential.helper store 2>/dev/null || true
git config --global core.autocrlf input 2>/dev/null || true
git config --global color.ui auto 2>/dev/null || true
git config --global safe.directory '*' 2>/dev/null || true

# 4. 配置 Claude Code（如果配置文件不存在）
# # 4. Configure Claude Code (if API key provided or config missing)
# if [ -n "$ANTHROPIC_API_KEY" ] && [ ! -f ${HOME}/.claude/config.json ]; then
#     echo -e "${YELLOW}Creating Claude config...${NC}"
#     mkdir -p ${HOME}/.claude
#     cat > ${HOME}/.claude/config.json <<EOF
# {
#   "apiKey": "${ANTHROPIC_API_KEY}",
#   "defaultModel": "claude-sonnet-4.5",
#   "workspace": "${HOME}/workspace"
# }
# EOF
#     chmod 600 ${HOME}/.claude/config.json
# fi

# 5. Create example project if none exist (fixes "no projects found")
mkdir -p ${HOME}/.claude/projects/-home-claudeui-workspace-deleteme

# # 5. 配置 Cursor（如果配置文件不存在）
# if [ ! -f ${HOME}/.cursor/config.json ]; then
#     echo -e "${YELLOW}Creating Cursor config...${NC}"
#     mkdir -p ${HOME}/.cursor
#     cat > ${HOME}/.cursor/config.json <<EOF
# {
#   "workspace": "${HOME}/workspace",
#   "enableAI": true
# }
# EOF
#     chmod 600 ${HOME}/.cursor/config.json
# fi

# 6. 配置 Vim（如果配置文件不存在）
if [ ! -f ${HOME}/.vimrc ]; then
    echo -e "${YELLOW}Creating Vim config...${NC}"
    cat > ${HOME}/.vimrc <<EOF
set number
set autoindent
set tabstop=4
set shiftwidth=4
set expandtab
syntax on
set background=dark
set mouse=a
EOF
fi

# 7. 配置 Tmux（如果配置文件不存在）
if [ ! -f ${HOME}/.tmux.conf ]; then
    echo -e "${YELLOW}Creating Tmux config...${NC}"
    cat > ${HOME}/.tmux.conf <<EOF
set -g mouse on
set -g history-limit 10000
set -g base-index 1
setw -g pane-base-index 1
EOF
fi

# 8. 配置 Bash（如果配置文件不存在或需要更新）
if [ ! -f ${HOME}/.bashrc.custom ]; then
    echo -e "${YELLOW}Creating Bash custom config...${NC}"
    cat > ${HOME}/.bashrc.custom <<'EOF'
# Custom aliases
alias ll='ls -alF'
alias la='ls -A'
alias l='ls -CF'
alias ..='cd ..'
alias ...='cd ../..'
alias grep='grep --color=auto'
alias fgrep='fgrep --color=auto'
alias egrep='egrep --color=auto'

# Git aliases
alias gs='git status'
alias ga='git add'
alias gc='git commit'
alias gp='git push'
alias gl='git log --oneline --graph --all'
alias gd='git diff'

# Modern tools aliases
alias cat='bat --paging=never'
alias ls='exa'
alias find='fd'

# Claude Code UI paths
export CLAUDE_DATA_PATH="${HOME}/.claude-code-ui/data"
export CLAUDE_WORKSPACE="${HOME}/workspace"

# Go paths
export GOPATH="${HOME}/go"
export GOBIN="${HOME}/go/bin"
export PATH="${GOBIN}:${PATH}"

# History settings
export HISTSIZE=10000
export HISTFILESIZE=20000
export HISTCONTROL=ignoreboth:erasedups

# FZF configuration
export FZF_DEFAULT_COMMAND='fd --type f --hidden --follow --exclude .git'
export FZF_DEFAULT_OPTS='--height 40% --layout=reverse --border'
EOF
    
    # 添加到 .bashrc
    if ! grep -q ".bashrc.custom" ${HOME}/.bashrc 2>/dev/null; then
        echo "[ -f ${HOME}/.bashrc.custom ] && . ${HOME}/.bashrc.custom" >> ${HOME}/.bashrc
    fi
fi

# 9. 设置数据库路径环境变量
export DATABASE_PATH="${HOME}/.claude-code-ui/data/claude-code-ui.db"

# # 10. 创建符号链接（向后兼容）
# ln -snf ${HOME}/workspace ${HOME}/.claude-code-ui/workspace 2>/dev/null || true

# 11. 显示环境信息
echo -e "${GREEN}=== Environment Information ===${NC}"
echo "User: $(whoami)"
echo "Home: ${HOME}"
echo "Node.js: $(node --version)"
echo "npm: $(npm --version)"
echo "Go: $(go version | awk '{print $3}')"
echo "Java: $(java -version 2>&1 | head -n 1 | awk -F '"' '{print $2}')"
echo "Claude Code: $(claude --version 2>/dev/null || echo 'Not configured')"
echo "Python: $(python3 --version | awk '{print $2}')"
echo ""
echo "Working Directory: $(pwd)"
echo "Data Path: ${DATABASE_PATH}"
echo "Workspace: ${HOME}/workspace"
echo -e "${GREEN}===============================${NC}"

# 12. 显示使用提示
cat <<EOF

${GREEN}=== Quick Start Guide ===${NC}
1. Web UI: http://localhost:${PORT}
2. Workspace: ${HOME}/workspace
3. Data: ${HOME}/.claude-code-ui/data

${YELLOW}Useful Commands:${NC}
  - claude-code-ui   : Start the UI server
  - claude          : Run Claude Code CLI
  - pm2 list        : List running processes
  - lazygit         : Git UI
  - htop            : System monitor

EOF

# 13. 启动应用
echo -e "${GREEN}Starting Claude Code UI...${NC}"
cd ${HOME}/workspace

# 使用 PM2 启动（如果需要后台运行）
if [ "$USE_PM2" = "true" ]; then
    exec pm2-runtime start claude-code-ui --name "claude-code-ui" -- --port ${PORT}
else
    exec claude-code-ui --port ${PORT} --database-path ~/.claude-code-ui/data/claude-code-ui.db "$@"
fi