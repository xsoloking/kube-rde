# Domain Configuration Guide

This guide explains how to configure domain names for KubeRDE in various deployment scenarios.

## Table of Contents

- [Overview](#overview)
- [Domain Configuration Patterns](#domain-configuration-patterns)
- [DNS Provider Setup](#dns-provider-setup)
- [TLS Certificate Configuration](#tls-certificate-configuration)
- [Updating Configuration](#updating-configuration)
- [Troubleshooting](#troubleshooting)

## Overview

KubeRDE requires domain configuration for:
1. **Main Web UI** - Access the management console
2. **Keycloak Authentication** - OAuth/OIDC authentication server
3. **Agent Subdomains** - Individual workspace access

### Why Domain Configuration Matters

- **Security**: Proper HTTPS configuration
- **Authentication**: OAuth redirects require valid URLs
- **Agent Routing**: Each workspace gets a unique subdomain
- **Cookie Sharing**: Single sign-on across services

## Domain Configuration Patterns

### Pattern 1: Single Domain (Simplest)

Best for: Development, testing, small deployments

```
Domain: kuberde.com
├── Main UI:    https://kuberde.com
├── Keycloak:   https://kuberde.com/auth (path-based)
└── Agents:     https://*.kuberde.com
                https://user-alice-dev.kuberde.com
                https://user-bob-jupyter.kuberde.com
```

**DNS Records:**
```
kuberde.com          A/CNAME  →  <ingress-ip>
*.kuberde.com        A/CNAME  →  <ingress-ip>
```

**Environment Variables:**
```bash
KUBERDE_PUBLIC_URL=https://kuberde.com
KUBERDE_AGENT_DOMAIN=kuberde.com
KEYCLOAK_URL=https://kuberde.com/auth
KEYCLOAK_PUBLIC_URL=https://kuberde.com/auth
```

**Pros:**
- Simple DNS setup (2 records)
- Single TLS certificate
- Easy to understand

**Cons:**
- Path-based routing for Keycloak
- Shared cookie domain

### Pattern 2: Subdomain-Based (Recommended)

Best for: Production, better organization

```
Domain: kuberde.com
├── Main UI:    https://kuberde.com
├── Keycloak:   https://sso.kuberde.com (dedicated subdomain)
└── Agents:     https://*.kuberde.com
                https://user-alice-dev.kuberde.com
```

**DNS Records:**
```
kuberde.com          A/CNAME  →  <ingress-ip>
*.kuberde.com        A/CNAME  →  <ingress-ip>
sso.kuberde.com      A/CNAME  →  <ingress-ip>
```

**Environment Variables:**
```bash
KUBERDE_PUBLIC_URL=https://kuberde.com
KUBERDE_AGENT_DOMAIN=kuberde.com
KEYCLOAK_URL=https://sso.kuberde.com
KEYCLOAK_PUBLIC_URL=https://sso.kuberde.com
```

**Pros:**
- Clean separation of services
- Dedicated subdomain for auth
- Professional URL structure

**Cons:**
- One extra DNS record
- Slightly more complex

### Pattern 3: Multi-Domain (Enterprise)

Best for: Large organizations, compliance requirements

```
Domains:
├── Main UI:    https://kuberde.company.com
├── Keycloak:   https://auth.company.com
└── Agents:     https://*.workspaces.company.com
                https://user-alice-dev.workspaces.company.com
```

**DNS Records:**
```
kuberde.company.com          A/CNAME  →  <ingress-ip>
auth.company.com             A/CNAME  →  <ingress-ip>
*.workspaces.company.com     A/CNAME  →  <ingress-ip>
```

**Environment Variables:**
```bash
KUBERDE_PUBLIC_URL=https://kuberde.company.com
KUBERDE_AGENT_DOMAIN=workspaces.company.com
KEYCLOAK_URL=https://auth.company.com
KEYCLOAK_PUBLIC_URL=https://auth.company.com
```

**Pros:**
- Complete separation
- Can use different TLS certificates
- Enhanced security policies

**Cons:**
- More complex DNS setup
- Multiple TLS certificates needed
- Cookie domain considerations

## DNS Provider Setup

### Cloudflare

1. **Add Domain to Cloudflare:**
   ```
   Websites → Add a Site → Enter your domain
   ```

2. **Create DNS Records:**
   ```
   DNS → Add record

   Type: A
   Name: @                    # kuberde.com
   IPv4: <your-ingress-ip>
   Proxy: DNS only (gray cloud)

   Type: A
   Name: *                    # *.kuberde.com
   IPv4: <your-ingress-ip>
   Proxy: DNS only (gray cloud)
   ```

3. **SSL/TLS Mode:**
   ```
   SSL/TLS → Overview → Full (strict)
   ```

**CLI Method:**
```bash
# Install cloudflare CLI
npm install -g cloudflare-cli

# Add records
cloudflare add @ A <ingress-ip>
cloudflare add "*" A <ingress-ip>
```

### AWS Route53

1. **Create Hosted Zone:**
   ```bash
   aws route53 create-hosted-zone \
     --name kuberde.com \
     --caller-reference $(date +%s)
   ```

2. **Get Zone ID:**
   ```bash
   ZONE_ID=$(aws route53 list-hosted-zones \
     --query "HostedZones[?Name=='kuberde.com.'].Id" \
     --output text | cut -d'/' -f3)
   ```

3. **Create Records:**
   ```bash
   # Main domain
   aws route53 change-resource-record-sets \
     --hosted-zone-id $ZONE_ID \
     --change-batch '{
       "Changes": [{
         "Action": "CREATE",
         "ResourceRecordSet": {
           "Name": "kuberde.com",
           "Type": "A",
           "TTL": 300,
           "ResourceRecords": [{"Value": "<ingress-ip>"}]
         }
       }]
     }'

   # Wildcard
   aws route53 change-resource-record-sets \
     --hosted-zone-id $ZONE_ID \
     --change-batch '{
       "Changes": [{
         "Action": "CREATE",
         "ResourceRecordSet": {
           "Name": "*.kuberde.com",
           "Type": "A",
           "TTL": 300,
           "ResourceRecords": [{"Value": "<ingress-ip>"}]
         }
       }]
     }'
   ```

### Google Cloud DNS

1. **Create DNS Zone:**
   ```bash
   gcloud dns managed-zones create kuberde-zone \
     --dns-name="kuberde.com." \
     --description="KubeRDE DNS Zone"
   ```

2. **Get Name Servers:**
   ```bash
   gcloud dns managed-zones describe kuberde-zone
   ```

3. **Create Records:**
   ```bash
   # Main domain
   gcloud dns record-sets create kuberde.com. \
     --zone=kuberde-zone \
     --type=A \
     --ttl=300 \
     --rrdatas=<ingress-ip>

   # Wildcard
   gcloud dns record-sets create "*.kuberde.com." \
     --zone=kuberde-zone \
     --type=A \
     --ttl=300 \
     --rrdatas=<ingress-ip>
   ```

### Azure DNS

1. **Create DNS Zone:**
   ```bash
   az network dns zone create \
     --resource-group <your-rg> \
     --name kuberde.com
   ```

2. **Create Records:**
   ```bash
   # Main domain
   az network dns record-set a add-record \
     --resource-group <your-rg> \
     --zone-name kuberde.com \
     --record-set-name @ \
     --ipv4-address <ingress-ip>

   # Wildcard
   az network dns record-set a add-record \
     --resource-group <your-rg> \
     --zone-name kuberde.com \
     --record-set-name "*" \
     --ipv4-address <ingress-ip>
   ```

### Domain Registrar (Generic)

For any domain registrar (GoDaddy, Namecheap, etc.):

1. **Login to your registrar**
2. **Go to DNS Management**
3. **Add A Records:**
   ```
   Type    Name    Value               TTL
   A       @       <ingress-ip>        300
   A       *       <ingress-ip>        300
   ```

## TLS Certificate Configuration

### Option 1: cert-manager with Let's Encrypt (Recommended)

**Install cert-manager:**
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

**Create ClusterIssuer:**
```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
```

**Annotate Ingress:**
```yaml
metadata:
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - kuberde.com
    - "*.kuberde.com"
    secretName: kuberde-tls
```

### Option 2: Cloud-Managed Certificates

#### GCP: Google-managed Certificates

```yaml
apiVersion: networking.gke.io/v1
kind: ManagedCertificate
metadata:
  name: kuberde-cert
spec:
  domains:
    - kuberde.com
    - "*.kuberde.com"
```

#### AWS: ACM Certificates

```bash
# Request certificate
aws acm request-certificate \
  --domain-name kuberde.com \
  --subject-alternative-names "*.kuberde.com" \
  --validation-method DNS

# Use in ALB Ingress
annotations:
  alb.ingress.kubernetes.io/certificate-arn: <cert-arn>
```

### Option 3: Manual Certificates

If you have existing certificates:

```bash
kubectl create secret tls kuberde-tls \
  --cert=path/to/cert.crt \
  --key=path/to/cert.key \
  -n kuberde
```

## Updating Configuration

### Scenario: Changing Domain

If you need to change from `kuberde.com` to `new-domain.com`:

**1. Update DNS Records:**
```bash
# Add new domain records
# Keep old domain active during transition
```

**2. Update KubeRDE Server:**
```bash
kubectl set env deployment/kuberde-server \
  -n kuberde \
  KUBERDE_PUBLIC_URL=https://new-domain.com \
  KUBERDE_AGENT_DOMAIN=new-domain.com
```

**3. Update Keycloak:**
```bash
kubectl set env deployment/kuberde-keycloak \
  -n kuberde \
  KC_HOSTNAME=new-domain.com
```

**4. Update Ingress:**
```bash
# Edit ingress resource
kubectl edit ingress kuberde-ingress -n kuberde

# Update hosts section
hosts:
  - new-domain.com
  - "*.new-domain.com"
```

**5. Request New Certificate:**
```bash
# cert-manager will automatically request new cert
# if Ingress hosts change

# Or manually trigger
kubectl delete certificate kuberde-tls -n kuberde
```

**6. Update CLI Configuration:**
```bash
# Users need to update their CLI config
kuberde-cli config set server https://new-domain.com
```

### Using Helm

If deployed with Helm:

```bash
helm upgrade kuberde ./charts/kuberde \
  --namespace kuberde \
  --set global.domain=new-domain.com \
  --set global.publicUrl=https://new-domain.com \
  --reuse-values
```

## Troubleshooting

### DNS Not Resolving

**Check DNS propagation:**
```bash
# Using nslookup
nslookup kuberde.com
nslookup user-test.kuberde.com

# Using dig
dig kuberde.com
dig @8.8.8.8 kuberde.com  # Check against Google DNS

# Online tools
# https://dnschecker.org
```

**Common issues:**
- **TTL**: Wait for TTL to expire (usually 300 seconds)
- **Propagation**: Can take up to 48 hours globally
- **Caching**: Clear local DNS cache
  ```bash
  # macOS
  sudo dscacheutil -flushcache
  sudo killall -HUP mDNSResponder

  # Linux
  sudo systemd-resolve --flush-caches

  # Windows
  ipconfig /flushdns
  ```

### Certificate Issues

**Certificate not issued:**
```bash
# Check certificate status
kubectl get certificate -n kuberde
kubectl describe certificate kuberde-tls -n kuberde

# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager

# Check challenges
kubectl get challenges -n kuberde
kubectl describe challenge -n kuberde <challenge-name>
```

**Common problems:**
- **Rate limits**: Let's Encrypt has rate limits (50 certs/week)
- **DNS validation**: Ensure DNS is correct
- **Firewall**: Port 80 must be accessible for HTTP-01 challenge

### Redirect Loops

If experiencing redirect loops:

**Check Ingress annotations:**
```yaml
# For NGINX Ingress
nginx.ingress.kubernetes.io/ssl-redirect: "true"

# For ALB
alb.ingress.kubernetes.io/ssl-redirect: "443"
```

**Check Keycloak proxy mode:**
```bash
# Should be "edge" for TLS termination at Ingress
KC_PROXY=edge
```

### Cookie Issues

If login doesn't persist:

**Check cookie domain:**
- Cookies must share root domain
- `kuberde.com` and `*.kuberde.com` share `.kuberde.com`
- `kuberde.com` and `agent.kuberde.com` can share `.kuberde.com`
- `kuberde.com` and `other.com` cannot share cookies

**Solution:**
Use Pattern 1 or 2 with shared root domain.

## Best Practices

1. **Use HTTPS in Production**
   - Always enable TLS
   - Use trusted certificates
   - Enforce SSL redirect

2. **Set Appropriate TTL**
   - Low TTL (300) for initial setup
   - Higher TTL (3600) after stable

3. **Test Before Going Live**
   - Test with staging domain first
   - Use Let's Encrypt staging for testing

4. **Monitor Certificate Expiry**
   - cert-manager auto-renews
   - Set up alerts for expiration

5. **Document Your Configuration**
   - Keep record of DNS setup
   - Document environment variables
   - Version control Ingress configs

## Next Steps

- [Configure TLS Certificates](../SECURITY.md#tls-configuration)
- [Set Up Monitoring](OPERATORS_RUNBOOK.md#monitoring)
- [Configure CLI](CLI_CONFIGURATION.md)

## Support

- [GitHub Issues](https://github.com/xsoloking/kube-rde/issues)
- [Documentation](../INDEX.md)
- [Community Discussions](https://github.com/xsoloking/kube-rde/discussions)
