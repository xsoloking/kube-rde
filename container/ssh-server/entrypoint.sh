#!/bin/bash
set -e

echo "=== Starting SSH server setup ==="

# 确保 /root 目录权限正确（这很关键！）
chmod 755 /root
chown root:root /root

# 创建 .ssh 目录
mkdir -p /root/.ssh
chmod 700 /root/.ssh
chown root:root /root/.ssh

# 从环境变量写入 authorized_keys
if [ -n "$SSH_PUBLIC_KEY" ]; then
    echo "$SSH_PUBLIC_KEY" | tr -d '\n\r' > /root/.ssh/authorized_keys
    echo "" >> /root/.ssh/authorized_keys
    chmod 600 /root/.ssh/authorized_keys
    chown root:root /root/.ssh/authorized_keys
    
    echo "✓ SSH public key configured"
    cat /root/.ssh/authorized_keys
    echo "Key fingerprint:"
    ssh-keygen -lf /root/.ssh/authorized_keys
else
    echo "✗ WARNING: SSH_PUBLIC_KEY environment variable not set"
fi

# 显示配置
echo "=== Permissions check ==="
ls -ld /root
ls -la /root/.ssh/

# 启动 SSH 服务（调试模式）
echo "=== Starting SSHD ==="
exec /usr/sbin/sshd -D