# SSH 数据流与传输架构分析

> 文档日期：2026-03-22
> 覆盖范围：SSH 连接路径、DERP relay 机制、WebSocket+Yamux 对比、P2P 打洞可行性分析

---

## 目录

1. [当前 SSH 连接路径](#1-当前-ssh-连接路径)
2. [DERP Relay 机制详解](#2-derp-relay-机制详解)
3. [为什么用 DERP 替换 WebSocket+Yamux 做 SSH](#3-为什么用-derp-替换-websocketyamux-做-ssh)
4. [传输大文件的瓶颈分析](#4-传输大文件的瓶颈分析)
5. [P2P 打洞可行性分析](#5-p2p-打洞可行性分析)
6. [优化建议](#6-优化建议)

---

## 1. 当前 SSH 连接路径

### 1.1 两条路径（DERP 优先，WebSocket 兜底）

用户通过 SSH 配置文件连接 KubeRDE agent：

```
~/.ssh/config:
  Host kuberde-*
    ProxyCommand kuberde-cli connect %n
```

`kuberde-cli connect` 启动时先尝试 DERP relay，失败则降级到 WebSocket+Yamux：

```go
// cmd/cli/cmd/connect.go
if err := connectViaDERP(wsURL, token.AccessToken); err != nil {
    fmt.Fprintf(os.Stderr, "DERP relay unavailable (%v), using WebSocket relay.\n", err)
    connectViaWebSocket(wsURL, token.AccessToken)
}
```

### 1.2 路径 A：DERP Relay（正常路径）

```
SSH Client (本地)
    │ stdin/stdout
    ▼
kuberde-cli (ProxyCommand)
    │ TLS WebSocket
    ▼
frp.byai.uk/derp-pod/{podIP}
    │
[DERP Server] ← 运行在 server pod 内，无状态帧路由
    │ TLS WebSocket
    ▼
kuberde-agent sidecar (oracle1, kuberde-cnn namespace)
    │ TCP localhost
    ▼
SSH server workload 容器 :22
    │
    ▼
用户家目录 PVC (kuberde-dev01-bbc-24e22c68)
```

**关键特性**：
- SSH 流量**全程经过 frp.byai.uk 中转**，不是端到端直连
- DERP 用 WireGuard node keys（Curve25519 NaCl box）加密每一帧
- server 只能看到"从 pubKeyA 到 pubKeyB 的加密帧"，无法解密 SSH 内容
- server 做无状态的帧路由，不持有 SSH session 状态

### 1.3 路径 B：WebSocket+Yamux（兜底路径）

当 DERP 不可用时（agent 离线、DERP server 异常），CLI 退回 WebSocket+Yamux：

```
SSH Client
    │
kuberde-cli
    │ WebSocket to wss://frp.byai.uk/connect/{agentID}
    ▼
Server (Yamux 客户端侧)
    │ session.Open() → 新的 Yamux stream
    ▼
Agent (Yamux 服务端，通过已建立的 WebSocket 长连接)
    │ TCP localhost
    ▼
SSH server :22
```

这条路径 server **可以看到 SSH 明文**（TLS 终止在 server 上，Yamux 数据未加密）。

### 1.4 连接信令流程（DERP 路径）

```
CLI                    Server                  Agent
 │                       │                      │
 │─ GET /api/agent-      │                      │
 │    coordination/{id} ▶│                      │
 │◀─ {pubKey, derp_url} ─│                      │
 │                       │                      │
 │─ POST /api/agent-     │                      │
 │    coordination/{id}  │                      │
 │    /peer              │                      │
 │   {user_public_key} ─▶│                      │
 │                       │─ Yamux control msg ─▶│
 │                       │  {add_wireguard_peer} │
 │◀─ 200 OK ─────────────│                      │
 │                       │                      │
 │──────── TLS WebSocket ─▶ frp.byai.uk/derp-pod/{ip} ◀── TLS WebSocket ──│
 │                       [DERP 帧路由：按 pubKey 转发]                      │
 │◀══════════════════ 加密 SSH 数据流 ══════════════════════════════════════▶│
```

---

## 2. DERP Relay 机制详解

### 2.1 WireGuard Keys 的作用

项目中 WireGuard 密钥**仅用于加密**，没有建立真正的 WireGuard 隧道（没有 tun 设备，没有内核模块）：

```go
// pkg/wgtunnel/tunnel.go
// 每帧加密方式：NaCl box（Curve25519 + XSalsa20 + Poly1305）
relay.client.Send(r.peerKey, frame)  // DERP 内部用 peerKey 做 box 加密
```

密钥生命周期：
- **Agent**：启动时生成，存储在 `/var/lib/kuberde-agent/wg-key`，注册到 `POST /api/agent-coordination/{id}`
- **CLI**：首次运行时生成，存储在 `~/.kuberde/wg-key`，每次连接时注册

### 2.2 帧格式

```go
// pkg/wgtunnel/tunnel.go
const (
    msgData  byte = 0x01  // payload: raw TCP data
    msgClose byte = 0x03  // 无 payload，信号流关闭
)

// 帧结构：[msgType(1B)][payload...]
frame := make([]byte, 1+len(data))
frame[0] = msgType
copy(frame[1:], data)
```

### 2.3 DERP Server 的 Pod 亲和性

因为 DERP server 在内存中维护当前活跃的 WebSocket 连接（按 pubKey 索引），**CLI 和 Agent 必须连到同一个 server pod**：

```go
// cmd/server/main.go
func derpURLForAgent(agentID string) string {
    if rec, err := agentPodSessionRepo.GetByAgentID(agentID); err == nil {
        // Agent 已连接到 pod，让 CLI 也连同一个 pod
        return frpURL + "/derp-pod/" + rec.PodIP
    }
    return frpURL + "/derp"  // 兜底用任意 pod
}
```

路由方式：`/derp-pod/{podIP}` 请求到达任意 pod 后，若不是目标 pod，则反向代理到目标 pod。

### 2.4 读写 buffer 大小

```go
// CLI 侧 (BridgeStdio)
buf := make([]byte, 32*1024)   // 32KB per read

// Agent 侧 (AgentListener.Accept)
buf := make([]byte, 64*1024)   // 64KB per read
```

---

## 3. 为什么用 DERP 替换 WebSocket+Yamux 做 SSH

### 3.1 WebSocket+Yamux 的 HA 问题

Yamux session 绑定在**单个 pod 的内存**里：

```go
// agentSessions 是进程内 map，不跨 pod
agentSessions = make(map[string]*yamux.Session)
```

多副本场景下，用户请求打到"错误"的 pod 时，需要：
1. 查 PostgreSQL（`agent_pod_sessions` 表）找到持有 session 的 pod IP
2. 做 pod-to-pod HTTP 反向代理（`forwardToPod`）

每次 HTTP 请求都可能触发 DB 查询 + pod 转发，开销较大。

### 3.2 DERP 的改进

DERP server 本身**无状态**（只有当前活跃连接的内存索引，不持久化）：

```go
// DERP relay (embedded, stateless – safe for multi-replica HA)
derpSrv = derp.NewServer(serverNodeKey, tslogger.Discard)
```

路由决策**在连接建立时（客户端侧）完成**：CLI 拿到 `derp_url` 后直接连目标 pod，不是每次帧都转发。

| 维度 | WebSocket+Yamux | DERP |
|------|----------------|------|
| server 状态 | yamux.Session（内存）+ agent_pod_sessions（DB） | 只有活跃连接索引（内存，可丢） |
| 多副本路由 | 每次请求查 DB + pod-to-pod 转发 | 连接建立时一次性路由 |
| server 对 SSH 的可见性 | 可见（TLS 终止后明文） | 不可见（NaCl box 加密） |
| SSH session 中断（pod 重启） | 是 | 是（两种路径都断） |

### 3.3 HTTP 代理流量（Jupyter/Coder）仍走 Yamux

DERP 是纯帧转发，不支持 HTTP 协议感知。Jupyter、Coder、文件浏览器等 HTTP 代理**仍走 WebSocket+Yamux**：

```
┌─────────────────────────────────────────────────────┐
│ SSH 连接  → DERP relay（端到端加密，HA 较好）         │
│ HTTP 代理 → WebSocket+Yamux（server 可见，HA 有补丁）  │
└─────────────────────────────────────────────────────┘
```

---

## 4. 传输大文件的瓶颈分析

### 4.1 数据路径层级

```
SSH 应用层（AES-GCM / ChaCha20 加密）
  └── kuberde-cli BridgeStdio (read buf=32KB)
       └── DERP frame (1B header + payload)
            └── derphttp.Client.Send (每帧一次 WebSocket 消息)
                 └── TLS over TCP → frp.byai.uk
                      └── DERP Server 帧路由
                           └── TLS over TCP → agent
                                └── DERP recvFiltered (read buf=64KB)
                                     └── TCP write → :22
                                          └── SSH server → 文件系统
```

**双重加密**：SSH 加密一次 + NaCl box 一次（CPU 开销约 2x，现代硬件不是瓶颈）。

### 4.2 瓶颈排序

| 优先级 | 瓶颈 | 原因 |
|--------|------|------|
| **1（最主要）** | frp.byai.uk 服务器带宽 | 所有流量中转，上下行均消耗 |
| 2 | TCP Head-of-Line Blocking | DERP 底层用 TCP/WebSocket |
| 3 | 无流控机制 | 帧级转发，依赖 SSH 自身窗口 |
| 4 | 双重加密 CPU 开销 | 现代 CPU 影响较小 |

### 4.3 实际限速公式

```
实际速度 ≈ min(
    frp.byai.uk 上行带宽 / 并发连接数,
    用户本地网络下行速度,
    oracle1 出口带宽
)
```

---

## 5. P2P 打洞可行性分析

### 5.1 为什么当前不是 P2P

DERP 是**服务器辅助中继**，不是点对点。Tailscale 完整 P2P 方案需要 STUN/ICE 打洞（NAT traversal），本项目当前只实现了 DERP relay 部分。

### 5.2 ICE 打洞的基本原理

ICE 收集三类候选地址：

```
Agent pod (oracle1)               User (家里)
┌─────────────────────┐          ┌──────────────────┐
│ host: 10.42.0.207   │          │ host: 192.168.1.5│
│ srflx: 45.1.2.3:x  │  ←STUN→  │ srflx: 1.2.3.4:y │
│ relay: derp url     │          │ relay: derp url   │
└─────────────────────┘          └──────────────────┘
          逐一尝试连通性，成功则直连
```

### 5.3 NAT 类型对打洞的影响

| NAT 类型 | 描述 | 打洞是否可行 |
|----------|------|-------------|
| Full Cone | 任何外部主机可访问映射端口 | ✅ 可行 |
| Restricted Cone | 只允许发过包的外部主机访问 | ✅ 可行 |
| Port Restricted Cone | 限制 IP + 端口 | ✅ 可行 |
| **Symmetric NAT** | 每次新连接分配不同外部端口 | ❌ 打洞失败 |

### 5.4 k8s Pod 的 NAT 叠加问题

KubeRDE agent 是 k8s pod，出站流量经过**多层 NAT**：

```
pod 10.42.0.207:UDP随机
    ↓ iptables MASQUERADE (k8s CNI，Symmetric NAT 行为)
node 172.16.1.3:另一随机端口
    ↓ IDC/云厂商 SNAT（可能再有一层）
公网 45.1.2.3:又一随机端口
```

**iptables MASQUERADE 等效于 Symmetric NAT**：STUN 看到的端口与打洞尝试时实际分配的端口不同，包被 NAT 丢弃。

### 5.5 各场景分析

| 场景 | 用户侧 NAT | Agent 侧 NAT | 打洞结果 |
|------|-----------|-------------|---------|
| 家里 → k8s pod (普通部署) | Cone | Symmetric (iptables) | ❌ 失败 |
| 家里 → k8s pod (hostNetwork:true + 节点有公网IP) | Cone | 单层 Cone | ✅ 可能成功 |
| 公司（对称NAT）→ k8s pod | Symmetric | Symmetric | ❌ 必败 |
| 家里 → 云主机直接公网IP（非pod） | Cone | 无NAT/单层 | ✅ 成功 |

**结论：KubeRDE 的 agent 以 k8s pod 形式运行，受 iptables masquerade 影响，ICE 打洞成功率极低（<10%），几乎总是退回 DERP relay。**

### 5.6 P2P 方案的技术选型（备选，未实施）

若未来要实现打洞，推荐使用 **pion/ice**（Go，MIT 许可，可商用）：

| 方案 | License | 是否需要 WireGuard | 成本估算 |
|------|---------|-----------------|---------|
| pion/ice + 现有 NaCl 加密 | MIT ✅ | 否 | ~8 工作日 |
| pion/webrtc DataChannel | MIT ✅ | 否 | ~12 工作日（偏重） |
| netbird | BSD-3 ✅ | 是 | 作为独立网络层，嵌入复杂 |
| Tailscale magicsock | BSL ⚠️ | 是 | 商用需评估许可证 |

**ICE 不需要 WireGuard**。WireGuard 是传输加密协议，ICE 是 NAT 穿透机制，两者正交。现有的 Curve25519/NaCl 加密可以直接用在 ICE 建立的 UDP 通道上。

但因为 k8s pod 的 Symmetric NAT 特性，**对 KubeRDE 场景做 ICE 打洞的性价比极低**，不建议实施。

---

## 6. 优化建议

### 6.1 零改造成本（立即可做）

增大 read buffer，对大文件（scp/rsync）有明显改善：

```go
// pkg/wgtunnel/tunnel.go
// CLI 侧：32KB → 256KB
buf := make([]byte, 256*1024)

// Agent 侧：64KB → 256KB
buf := make([]byte, 256*1024)
```

### 6.2 中等成本：DERP over QUIC

将 DERP 的底层传输从 WebSocket/TCP 改为 QUIC（`quic-go`，MIT），消除 TCP Head-of-Line Blocking：

- 适合大文件传输场景
- 不需要修改 DERP 协议，只换底层 transport
- `quic-go` MIT 许可，可商用
- 估计工作量：~2 周

### 6.3 不推荐：ICE 打洞

原因：k8s pod 的 iptables MASQUERADE 导致打洞成功率极低，8 天工作量换来<10% 的直连率，不划算。DERP 兜底仍是必须的。

---

## 附：当前使用的核心依赖版本

| 依赖 | 版本 | 用途 |
|------|------|------|
| `tailscale.com` | v1.50.1 | DERP server/client 实现 |
| `github.com/hashicorp/yamux` | — | WebSocket 多路复用（HTTP 代理路径） |
| `github.com/tailscale/wireguard-go` | v0.0.0-20250716 | WireGuard conn.Bind（当前未用于真实隧道） |

> **注意**：`tailscale.com v1.50.1` API 签名与更新版本不兼容（`derphttp.NewClient` 参数数量等），升级前需验证。
