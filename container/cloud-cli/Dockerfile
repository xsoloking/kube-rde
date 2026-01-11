# 使用 Node.js LTS (基于 Debian 最新稳定版)
FROM node:lts-bookworm

# 设置标签
LABEL maintainer="Derek <derek@example.com>"
LABEL description="Claude Code UI with full development tools"
LABEL version="1.0"



# 设置环境变量
ENV DEBIAN_FRONTEND=noninteractive \
    NODE_ENV=production \
    PORT=3001 \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8

# 更新系统并安装基础依赖
RUN apt-get update && apt-get install -y \
    # 基础工具
    curl \
    wget \
    ca-certificates \
    gnupg \
    lsb-release \
    apt-transport-https \
    software-properties-common \
    # 构建工具
    build-essential \
    python3 \
    python3-pip \
    python3-venv \
    cmake \
    pkg-config \
    # Git 和版本控制
    git \
    git-lfs \
    subversion \
    mercurial \
    # 文本处理工具
    vim \
    nano \
    emacs-nox \
    # 高级命令行工具
    ripgrep \
    fd-find \
    bat \
    exa \
    fzf \
    tree \
    jq \
    yq \
    # 网络工具
    netcat-openbsd \
    nmap \
    telnet \
    dnsutils \
    iputils-ping \
    traceroute \
    tcpdump \
    # 系统监控
    htop \
    iotop \
    nethogs \
    iftop \
    sysstat \
    lsof \
    strace \
    # 压缩工具
    zip \
    unzip \
    bzip2 \
    gzip \
    xz-utils \
    p7zip-full \
    # 数据库客户端
    postgresql-client \
    redis-tools \
    sqlite3 \
    # SSH 和安全
    openssh-client \
    openssh-server \
    gnupg2 \
    pass \
    # 其他实用工具
    tmux \
    screen \
    rsync \
    parallel \
    moreutils \
    dos2unix \
    procps \
    tini \
    gosu \
    && rm -rf /var/lib/apt/lists/*

# # 安装 Golang 最新稳定版 (1.24.x)
# RUN ARCH=$(uname -m) && \
#     if [ "$ARCH" = "x86_64" ]; then \
#         GO_ARCH="amd64"; \
#     elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then \
#         GO_ARCH="arm64"; \
#     else \
#         echo "Unsupported architecture: $ARCH" && exit 1; \
#     fi && \
#     wget -q https://go.dev/dl/go1.24.11.linux-${GO_ARCH}.tar.gz && \
#     tar -C /usr/local -xzf go1.24.11.linux-${GO_ARCH}.tar.gz && \
#     rm go1.24.11.linux-${GO_ARCH}.tar.gz

# ENV PATH="/usr/local/go/bin:${PATH}" 

# # 安装 OpenJDK 25 (Temurin distribution)
# RUN wget -O - https://packages.adoptium.net/artifactory/api/gpg/key/public | apt-key add - && \
#     echo "deb https://packages.adoptium.net/artifactory/deb $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/adoptium.list && \
#     apt-get update && \
#     apt-get install -y temurin-25-jdk && \
#     rm -rf /var/lib/apt/lists/*

# RUN ARCH=$(uname -m) && \
#     if [ "$ARCH" = "x86_64" ]; then \
#         ln -s /usr/lib/jvm/temurin-25-jdk-amd64 /usr/lib/jvm/default-jvm; \
#     elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then \
#         ln -s /usr/lib/jvm/temurin-25-jdk-arm64 /usr/lib/jvm/default-jvm; \
#     fi

# ENV JAVA_HOME="/usr/lib/jvm/default-jvm"
# ENV PATH="${JAVA_HOME}/bin:${PATH}"

# 创建符号链接以便于使用 fd 和 bat
RUN ln -s $(which fdfind) /usr/local/bin/fd 2>/dev/null || true && \
    ln -s $(which batcat) /usr/local/bin/bat 2>/dev/null || true

# 安装 Claude Code CLI (原生方式)
RUN npm install -g @anthropic-ai/claude-code 

# 全局安装 npm 工具
RUN npm install -g \
    @siteboon/claude-code-ui \
    pm2 \
    typescript \
    ts-node \
    tsx \
    nodemon \
    eslint \
    prettier \
    pnpm \
    commitizen \
    husky \
    lint-staged \
    npm-check-updates \
    http-server \
    serve \
    tldr \
    gtop \
    && npm cache clean --force

# 安装 Codex
RUN npm i -g @openai/codex 2>/dev/null || echo "Codex 需要手动安装或不可用"

# # 安装 Python 常用工具
# RUN pip3 install --no-cache-dir --break-system-packages \
#     ipython \
#     black \
#     pylint \
#     flake8 \
#     autopep8 \
#     pytest \
#     mypy \
#     poetry \
#     pipenv

# # 安装一些有用的 Go 工具
# RUN go install github.com/jesseduffield/lazygit@latest && \
#     go install github.com/charmbracelet/glow@latest && \
#     go install github.com/cli/cli/v2/cmd/gh@latest

# 创建非 root 用户
RUN groupadd -g 1001 claudeui && \
    useradd -u 1001 -g claudeui -m -s /bin/bash claudeui && \
    echo "claudeui ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# 配置 SSH
RUN mkdir -p /run/sshd && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config && \
    echo "PasswordAuthentication yes" >> /etc/ssh/sshd_config

# 切换到 claudeui 用户
USER claudeui
ENV HOME=/home/claudeui

# # 设置用户环境变量
# ENV PATH="${HOME}/.local/bin:${PATH}" \
#     GOPATH="${HOME}/go" \
#     GOBIN="${HOME}/go/bin"
# ENV PATH="${GOBIN}:${PATH}"

# 暴露端口
EXPOSE 3001

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD curl -f http://localhost:${PORT}/health || exit 1

# 复制启动脚本
USER root
COPY docker-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh && \
    chown claudeui:claudeui /entrypoint.sh

# USER claudeui


# 使用 tini 作为入口点
ENTRYPOINT ["/usr/bin/tini", "--", "/entrypoint.sh"]
CMD []