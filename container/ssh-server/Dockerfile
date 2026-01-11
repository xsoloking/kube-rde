FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# 安装 OpenSSH Server
RUN apt-get update && \
    apt-get install -y openssh-server && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# 创建 SSH 运行目录
RUN mkdir -p /var/run/sshd

# 配置 SSH 服务
RUN sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config && \
    sed -i 's/#PubkeyAuthentication yes/PubkeyAuthentication yes/' /etc/ssh/sshd_config

# 复制启动脚本
COPY container/ssh-server/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# 暴露 SSH 端口
EXPOSE 22

# 使用启动脚本
ENTRYPOINT ["/entrypoint.sh"]