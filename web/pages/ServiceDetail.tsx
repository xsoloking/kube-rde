import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';

import { servicesApi, Service, systemConfigApi, SystemConfig } from '../services/api';

const ServiceDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();

  const [service, setService] = useState<Service | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copiedLogin, setCopiedLogin] = useState(false);
  const [copiedSSH, setCopiedSSH] = useState(false);
  const [copiedSSHConfig, setCopiedSSHConfig] = useState(false);
  const [copiedWebURL, setCopiedWebURL] = useState(false);
  const [logs, setLogs] = useState<string>('');
  const [logsContainer, setLogsContainer] = useState<'kuberde-agent' | 'workload'>('workload');
  const [logsLoading, setLogsLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [serviceLoaded, setServiceLoaded] = useState(false);
  const [systemConfig, setSystemConfig] = useState<SystemConfig | null>(null);

  // Fetch system config on mount
  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const config = await systemConfigApi.get();
        setSystemConfig(config);
      } catch (err) {
        console.error('Failed to fetch system config:', err);
        // Fallback to default values if config fetch fails
        setSystemConfig({
          public_url: 'https://frp.byai.uk',
          agent_domain: 'frp.byai.uk',
          keycloak_url: 'https://sso.byai.uk',
          realm_name: 'kuberde',
        });
      }
    };

    fetchConfig();
  }, []);

  useEffect(() => {
    const fetchService = async () => {
      if (!id) return;

      try {
        setLoading(true);
        setError(null);
        const data = await servicesApi.get(id);
        setService(data);
        setServiceLoaded(true);
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to load service';
        setError(message);
        console.error('Error fetching service:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchService();
  }, [id]);

  // Fetch logs when service is loaded or container changes
  useEffect(() => {
    const fetchLogs = async () => {
      if (!id || !serviceLoaded) return;

      setLogsLoading(true);
      try {
        const response = await servicesApi.getLogs(id, logsContainer);
        setLogs(response.logs || 'No logs available');
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to fetch logs';
        setLogs(`Error fetching logs: ${message}`);
      } finally {
        setLogsLoading(false);
      }
    };

    fetchLogs();
  }, [id, logsContainer, serviceLoaded]);

  if (loading || !systemConfig) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="text-center">
          <div className="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-primary mb-4"></div>
          <p className="text-text-secondary">Loading service...</p>
        </div>
      </div>
    );
  }

  if (error || !service) {
    return (
      <div className="p-8 lg:p-12 max-w-[1400px] mx-auto">
        <div className="rounded-xl bg-red-500/10 border border-red-500/20 p-6">
          <p className="text-red-500 font-semibold">{error || 'Service not found'}</p>
        </div>
      </div>
    );
  }

  const isSSH = service.agent_type?.toLowerCase().trim() === 'ssh';

  // Helper to convert public_url to WebSocket URL
  const getWebSocketURL = () => {
    if (!systemConfig) return 'wss://frp.byai.uk';
    const url = systemConfig.public_url.replace('https://', 'wss://').replace('http://', 'ws://');
    return url;
  };

  const getServiceIcon = () => {
    switch (service.agent_type) {
      case 'ssh':
        return 'terminal';
      case 'file':
        return 'folder';
      case 'coder':
        return 'code';
      case 'jupyter':
        return 'science';
      default:
        return 'deployed_code';
    }
  };

  const getStatusColor = () => {
    switch (service.status) {
      case 'running':
        return 'emerald';
      case 'starting':
        return 'blue';
      case 'stopped':
        return 'red';
      case 'error':
        return 'orange';
      default:
        return 'gray';
    }
  };

  const handleDownloadCLI = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();

    // Detect user's OS and architecture
    const platform = navigator.platform.toLowerCase();
    const userAgent = navigator.userAgent.toLowerCase();
    let platformId = '';

    // Detect OS
    if (platform.includes('mac') || userAgent.includes('mac')) {
      // Detect Apple Silicon vs Intel
      if (userAgent.includes('arm') || platform.includes('arm')) {
        platformId = 'darwin-arm64';
      } else {
        platformId = 'darwin-amd64';
      }
    } else if (platform.includes('win') || userAgent.includes('win')) {
      // Detect Windows architecture
      if (userAgent.includes('wow64') || userAgent.includes('win64')) {
        platformId = 'windows-amd64';
      } else {
        platformId = 'windows-amd64'; // Default to amd64 for Windows
      }
    } else {
      // Linux or other Unix-like
      if (userAgent.includes('arm') || platform.includes('arm')) {
        platformId = 'linux-arm64';
      } else {
        platformId = 'linux-amd64';
      }
    }

    try {
      // Use fetch to download the binary
      const response = await fetch(`/download/cli/${platformId}`, {
        method: 'GET',
        credentials: 'include',
      });

      if (!response.ok) {
        throw new Error(`Download failed: ${response.statusText}`);
      }

      // Get the blob
      const blob = await response.blob();

      // Create download link
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `kuberde-cli-${platformId}${platformId.includes('windows') ? '.exe' : ''}`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Download failed:', error);
      alert('Failed to download kuberde-cli. Please try again.');
    }
  };

  const copyToClipboard = async (text: string, setCopiedState: (value: boolean) => void) => {
    try {
      // Check if Clipboard API is available (HTTPS/localhost only)
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
        setCopiedState(true);
        setTimeout(() => setCopiedState(false), 2000);
      } else {
        // Fallback for HTTP contexts: use legacy execCommand
        const textArea = document.createElement('textarea');
        textArea.value = text;
        textArea.style.position = 'fixed';
        textArea.style.left = '-999999px';
        textArea.style.top = '-999999px';
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();

        const successful = document.execCommand('copy');
        document.body.removeChild(textArea);

        if (successful) {
          setCopiedState(true);
          setTimeout(() => setCopiedState(false), 2000);
        } else {
          throw new Error('execCommand copy failed');
        }
      }
    } catch (err) {
      console.error('Failed to copy:', err);
      alert('Copy failed. Please copy manually.');
    }
  };

  const handleRestart = async () => {
    if (!id || !window.confirm('Are you sure you want to restart this service?')) return;

    setActionLoading(true);
    try {
      await servicesApi.restart(id);
      alert('Service restart initiated. The pod will be recreated shortly.');
      // Refresh service data
      const data = await servicesApi.get(id);
      setService(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to restart service';
      alert(`Error: ${message}`);
    } finally {
      setActionLoading(false);
    }
  };

  const handleStop = async () => {
    if (
      !id ||
      !window.confirm(
        'Are you sure you want to stop this service? This will scale the deployment to 0.',
      )
    )
      return;

    setActionLoading(true);
    try {
      await servicesApi.stop(id);
      alert('Service stopped successfully.');
      // Refresh service data
      const data = await servicesApi.get(id);
      setService(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to stop service';
      alert(`Error: ${message}`);
    } finally {
      setActionLoading(false);
    }
  };

  const handleStart = async () => {
    if (
      !id ||
      !window.confirm(
        'Are you sure you want to start this service? This will scale the deployment to 1.',
      )
    )
      return;

    setActionLoading(true);
    try {
      await servicesApi.start(id);
      alert('Service started successfully.');
      // Refresh service data
      const data = await servicesApi.get(id);
      setService(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to start service';
      alert(`Error: ${message}`);
    } finally {
      setActionLoading(false);
    }
  };

  const handlePinToggle = async () => {
    if (!id || !service) return;

    setActionLoading(true);
    try {
      const newPinnedState = !service.is_pinned;
      await servicesApi.update(id, { is_pinned: newPinnedState });
      setService({ ...service, is_pinned: newPinnedState });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to update pin status';
      alert(`Error: ${message}`);
    } finally {
      setActionLoading(false);
    }
  };

  const statusColor = getStatusColor();
  const isStopped = service.status === 'stopped';

  return (
    <div className="p-8 lg:p-10 max-w-7xl mx-auto space-y-10 animate-fade-in pb-20">
      {/* Breadcrumbs */}
      <nav className="flex items-center gap-3 text-xs font-medium">
        <Link
          to="/workspaces"
          className="text-text-secondary hover:text-white transition-colors text-[10px] uppercase tracking-widest font-bold"
        >
          Workspaces
        </Link>
        <span className="text-text-secondary">/</span>
        <Link
          to={`/workspaces/${service.workspace_id}`}
          className="text-text-secondary hover:text-white transition-colors text-[10px] uppercase tracking-widest font-bold"
        >
          Workspace
        </Link>
        <span className="text-text-secondary">/</span>
        <span className="bg-surface-highlight text-white px-2 py-0.5 rounded text-[10px] font-mono">
          {service.name}
        </span>
      </nav>

      {/* Header Section */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
        <div className="flex items-start gap-5">
          <div className="size-14 bg-surface-dark border border-border-dark rounded-xl flex items-center justify-center shadow-lg">
            <span
              className={`material-symbols-outlined text-3xl ${isSSH ? 'text-emerald-500' : 'text-primary'}`}
            >
              {getServiceIcon()}
            </span>
          </div>
          <div>
            <div className="flex items-center gap-4">
              <h1 className="text-3xl font-bold tracking-tight text-white">{service.name}</h1>
              <span
                className={`px-2.5 py-0.5 rounded-full bg-${statusColor}-500/10 text-${statusColor}-400 text-[10px] font-bold uppercase tracking-widest border border-${statusColor}-500/20 flex items-center gap-1.5`}
              >
                <span
                  className={`size-1.5 rounded-full bg-${statusColor}-500 ${service.status === 'running' ? 'animate-pulse' : ''}`}
                ></span>
                {service.status}
              </span>
            </div>
            <div className="flex flex-wrap items-center gap-x-6 gap-y-2 mt-2 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
              <span className="flex items-center gap-2">
                <span className="material-symbols-outlined text-[16px] text-text-secondary">
                  fingerprint
                </span>
                ID: {service.id}
              </span>
              {service.agent_type && (
                <span className="flex items-center gap-2">
                  <span className="material-symbols-outlined text-[16px] text-text-secondary">
                    category
                  </span>
                  Type: {service.agent_type.toUpperCase()}
                </span>
              )}
              <span className="flex items-center gap-2">
                <span className="material-symbols-outlined text-[16px] text-text-secondary">
                  dns
                </span>
                Port: {service.external_port}
              </span>
              {service.ttl && (
                <span className="flex items-center gap-2">
                  <span className="material-symbols-outlined text-[16px] text-text-secondary">
                    schedule
                  </span>
                  TTL: {service.ttl === '0' ? 'Disabled' : service.ttl}
                </span>
              )}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={handlePinToggle}
            disabled={actionLoading}
            className={`flex items-center gap-2 px-4 py-2 text-[10px] font-bold uppercase tracking-widest rounded-lg border transition-all disabled:opacity-50 disabled:cursor-not-allowed ${
              service.is_pinned
                ? 'bg-primary/10 text-primary border-primary/20 hover:bg-primary/20'
                : 'bg-surface-highlight text-text-secondary border-border-dark hover:text-white hover:bg-white/10'
            }`}
            title={
              service.is_pinned
                ? 'Unpin from Workspace Card'
                : 'Pin to Workspace Card for quick access'
            }
          >
            <span
              className={`material-symbols-outlined text-[18px] ${service.is_pinned ? 'icon-fill' : ''}`}
            >
              push_pin
            </span>
            {service.is_pinned ? 'Pinned' : 'Pin to Card'}
          </button>
          <button
            onClick={handleRestart}
            disabled={actionLoading || isStopped}
            className="flex items-center gap-2 px-4 py-2 bg-surface-highlight hover:bg-white/10 text-white text-[10px] font-bold uppercase tracking-widest rounded-lg border border-border-dark transition-all disabled:opacity-50 disabled:cursor-not-allowed"
            title={
              isStopped
                ? 'Cannot restart a stopped service. Please start it first.'
                : 'Restart service by deleting the pod'
            }
          >
            <span className="material-symbols-outlined text-[18px]">restart_alt</span>
            {actionLoading ? 'Processing...' : 'Restart'}
          </button>
          {isStopped ? (
            <button
              onClick={handleStart}
              disabled={actionLoading}
              className="flex items-center gap-2 px-4 py-2 bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 text-[10px] font-bold uppercase tracking-widest rounded-lg border border-emerald-500/20 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <span className="material-symbols-outlined text-[18px]">play_circle</span>
              {actionLoading ? 'Processing...' : 'Start'}
            </button>
          ) : (
            <button
              onClick={handleStop}
              disabled={actionLoading}
              className="flex items-center gap-2 px-4 py-2 bg-red-500/10 hover:bg-red-500/20 text-red-400 text-[10px] font-bold uppercase tracking-widest rounded-lg border border-red-500/20 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <span className="material-symbols-outlined text-[18px]">stop_circle</span>
              {actionLoading ? 'Processing...' : 'Stop'}
            </button>
          )}
        </div>
      </div>

      {/* Connect Section - Unified background for consistency */}
      <div className="space-y-6">
        <h3 className="flex items-center gap-2 text-primary font-bold text-sm uppercase tracking-widest">
          <span className="material-symbols-outlined icon-fill">link</span>
          Connect
        </h3>

        <div className="space-y-6">
          {/* Conditional Access Area */}
          {isSSH ? (
            <>
              {/* Step 1: SSH Access - Get the CLI */}
              <div className="bg-surface-dark rounded-xl border border-border-dark overflow-hidden shadow-xl">
                <div className="p-8 space-y-6">
                  <div className="flex justify-between items-center">
                    <h4 className="font-bold text-xs uppercase tracking-widest flex items-center gap-2 text-white">
                      <span className="material-symbols-outlined text-primary text-[20px]">
                        terminal
                      </span>
                      SSH Access
                    </h4>
                    <span className="text-[9px] font-bold text-white/40 uppercase tracking-widest">
                      Step 1
                    </span>
                  </div>
                  <div className="space-y-4">
                    <p className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                      Get the CLI
                    </p>
                    <div className="flex items-center justify-between bg-background-dark/50 border border-border-dark p-3 rounded-lg">
                      <div className="flex items-center gap-3">
                        <span className="material-symbols-outlined text-primary">download</span>
                        <span className="text-xs font-mono text-white/80">kuberde-cli-latest</span>
                      </div>
                      <button
                        onClick={handleDownloadCLI}
                        className="text-primary text-[10px] font-bold uppercase tracking-widest hover:underline transition-colors"
                      >
                        Download
                      </button>
                    </div>
                  </div>
                </div>
              </div>

              {/* Step 2: Quick Connect */}
              <div className="bg-surface-dark rounded-xl border border-border-dark overflow-hidden shadow-xl">
                <div className="p-8 space-y-6">
                  <div className="flex justify-between items-center">
                    <h4 className="font-bold text-xs uppercase tracking-widest flex items-center gap-2 text-white">
                      <span className="material-symbols-outlined text-emerald-500 text-[20px]">
                        bolt
                      </span>
                      Quick Connect
                    </h4>
                    <span className="text-[9px] font-bold text-white/40 uppercase tracking-widest">
                      Step 2
                    </span>
                  </div>

                  {/* Login Command */}
                  <div className="space-y-3">
                    <p className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                      1. Login via OIDC (Browser Authentication)
                    </p>
                    <div className="bg-background-dark/50 border border-border-dark p-4 rounded-lg group relative">
                      <code className="text-xs text-emerald-400 font-mono block leading-relaxed pr-8">
                        ./kuberde-cli login --issuer {systemConfig.keycloak_url}/realms/
                        {systemConfig.realm_name}
                      </code>
                      <button
                        onClick={() =>
                          copyToClipboard(
                            `./kuberde-cli login --issuer ${systemConfig.keycloak_url}/realms/${systemConfig.realm_name}`,
                            setCopiedLogin,
                          )
                        }
                        className="absolute top-4 right-4 text-text-secondary hover:text-white transition-colors"
                      >
                        <span className="material-symbols-outlined text-[18px]">
                          {copiedLogin ? 'check' : 'content_copy'}
                        </span>
                      </button>
                    </div>
                  </div>

                  {/* SSH Connect Command */}
                  <div className="space-y-3">
                    <p className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                      2. Connect to Agent via SSH ProxyCommand
                    </p>
                    <div className="bg-background-dark/50 border border-border-dark p-4 rounded-lg group relative">
                      <code className="text-xs text-emerald-400 font-mono block leading-relaxed pr-8">
                        {`ssh -o ProxyCommand="./kuberde-cli connect ${getWebSocketURL()}/connect/${service.agent_id}" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null root@placeholder`}
                      </code>
                      <button
                        onClick={() =>
                          copyToClipboard(
                            `ssh -o ProxyCommand="./kuberde-cli connect ${getWebSocketURL()}/connect/${service.agent_id}" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null root@placeholder`,
                            setCopiedSSH,
                          )
                        }
                        className="absolute top-4 right-4 text-text-secondary hover:text-white transition-colors"
                      >
                        <span className="material-symbols-outlined text-[18px]">
                          {copiedSSH ? 'check' : 'content_copy'}
                        </span>
                      </button>
                    </div>
                  </div>

                  {/* SSH Config - Optional */}
                  <div className="space-y-3 pt-4 border-t border-border-dark/50">
                    <div className="flex items-center gap-2">
                      <p className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                        3. Add to SSH Config (Optional)
                      </p>
                      <span className="px-2 py-0.5 bg-blue-500/10 text-blue-400 text-[9px] font-bold uppercase tracking-widest rounded border border-blue-500/20">
                        Recommended
                      </span>
                    </div>
                    <p className="text-[10px] text-text-secondary/70 leading-relaxed">
                      Add this configuration to your{' '}
                      <code className="text-xs bg-background-dark px-1.5 py-0.5 rounded font-mono text-white/80">
                        ~/.ssh/config
                      </code>{' '}
                      file for easier access
                    </p>
                    <div className="bg-background-dark/50 border border-border-dark p-4 rounded-lg group relative">
                      <code className="text-xs text-blue-400 font-mono block leading-relaxed pr-8 whitespace-pre">
                        {`Host ${service.agent_id.split('-ssh')[0]}
    HostName placeholder
    User root
    ProxyCommand ./kuberde-cli connect ${getWebSocketURL()}/connect/${service.agent_id}
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null`}
                      </code>
                      <button
                        onClick={() =>
                          copyToClipboard(
                            `Host ${service.agent_id.split('-ssh')[0]}\n    HostName placeholder\n    User root\n    ProxyCommand ./kuberde-cli connect ${getWebSocketURL()}/connect/${service.agent_id}\n    StrictHostKeyChecking no\n    UserKnownHostsFile /dev/null`,
                            setCopiedSSHConfig,
                          )
                        }
                        className="absolute top-4 right-4 text-text-secondary hover:text-white transition-colors"
                      >
                        <span className="material-symbols-outlined text-[18px]">
                          {copiedSSHConfig ? 'check' : 'content_copy'}
                        </span>
                      </button>
                    </div>
                    <div className="flex items-start gap-2 bg-blue-500/5 rounded-lg p-3 border border-blue-500/10">
                      <span className="material-symbols-outlined text-blue-400 text-[16px] mt-0.5 flex-shrink-0">
                        info
                      </span>
                      <p className="text-[10px] text-blue-400/80 leading-relaxed">
                        After adding this config, you can simply run:{' '}
                        <code className="text-xs bg-blue-500/10 px-1.5 py-0.5 rounded font-mono text-blue-300">
                          ssh {service.agent_id.split('-ssh')[0]}
                        </code>
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            </>
          ) : (
            <div className="bg-surface-dark rounded-xl border border-border-dark overflow-hidden shadow-xl">
              <div className="p-8 space-y-6">
                <div className="flex justify-between items-center">
                  <h4 className="font-bold text-xs uppercase tracking-widest flex items-center gap-2 text-white">
                    <span className="material-symbols-outlined text-primary text-[20px]">
                      language
                    </span>
                    Web Access
                  </h4>
                  <span className="text-[9px] font-bold text-primary bg-primary/10 px-2 py-0.5 rounded border border-primary/20 uppercase tracking-widest">
                    Public
                  </span>
                </div>
                <div className="space-y-4">
                  <p className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                    Public URL
                  </p>
                  <div className="flex gap-2">
                    <div className="flex-1 bg-background-dark/50 border border-border-dark p-3 rounded-lg flex items-center overflow-hidden">
                      <span className="text-xs font-mono text-white/80 truncate">
                        http://{service.remote_proxy || `${service.agent_id}.192-168-97-2.nip.io`}/
                      </span>
                    </div>
                    <button
                      onClick={() =>
                        copyToClipboard(
                          `http://${service.remote_proxy || `${service.agent_id}.192-168-97-2.nip.io`}/`,
                          setCopiedWebURL,
                        )
                      }
                      className="bg-background-dark border border-border-dark p-3 rounded-lg text-text-secondary hover:text-white transition-all shadow-sm"
                      title="Copy URL"
                    >
                      <span className="material-symbols-outlined text-[20px]">
                        {copiedWebURL ? 'check' : 'content_copy'}
                      </span>
                    </button>
                    <button
                      onClick={() =>
                        window.open(
                          `http://${service.remote_proxy || `${service.agent_id}.192-168-97-2.nip.io`}/`,
                          '_blank',
                        )
                      }
                      className="bg-primary text-white p-3 rounded-lg shadow-lg shadow-primary/20 hover:bg-primary-dark transition-all"
                      title="Open in new tab"
                    >
                      <span className="material-symbols-outlined text-[20px]">open_in_new</span>
                    </button>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Logs Section */}
      <div className="space-y-6">
        <h3 className="flex items-center gap-2 text-primary font-bold text-sm uppercase tracking-widest">
          <span className="material-symbols-outlined icon-fill">list_alt</span>
          Logs
        </h3>

        <div className="bg-surface-dark rounded-xl border border-border-dark overflow-hidden shadow-xl">
          <div className="px-6 py-4 border-b border-border-dark bg-background-dark/30 flex justify-between items-center">
            <div className="flex items-center gap-4">
              <h4 className="font-bold text-xs uppercase tracking-widest text-white/80">
                Container Logs
              </h4>
              <div className="flex gap-2">
                <button
                  onClick={() => setLogsContainer('workload')}
                  className={`px-3 py-1 text-[10px] font-bold uppercase tracking-widest rounded transition-all ${
                    logsContainer === 'workload'
                      ? 'bg-primary text-white'
                      : 'bg-background-dark text-text-secondary hover:text-white'
                  }`}
                >
                  Workload
                </button>
                <button
                  onClick={() => setLogsContainer('kuberde-agent')}
                  className={`px-3 py-1 text-[10px] font-bold uppercase tracking-widest rounded transition-all ${
                    logsContainer === 'kuberde-agent'
                      ? 'bg-primary text-white'
                      : 'bg-background-dark text-text-secondary hover:text-white'
                  }`}
                >
                  Agent
                </button>
              </div>
            </div>
            <button
              onClick={async () => {
                if (!id) return;
                setLogsLoading(true);
                try {
                  const response = await servicesApi.getLogs(id, logsContainer);
                  setLogs(response.logs || 'No logs available');
                } catch (err) {
                  const message = err instanceof Error ? err.message : 'Failed to fetch logs';
                  setLogs(`Error fetching logs: ${message}`);
                } finally {
                  setLogsLoading(false);
                }
              }}
              disabled={logsLoading}
              className="text-[10px] font-bold uppercase tracking-widest text-primary hover:underline disabled:opacity-50"
            >
              {logsLoading ? 'Loading...' : 'Refresh'}
            </button>
          </div>
          <div className="bg-background-dark/80 p-6 font-mono text-xs overflow-y-auto max-h-[400px] scrollbar-hide leading-relaxed">
            {logsLoading ? (
              <div className="flex items-center justify-center py-8 text-text-secondary">
                <div className="flex items-center gap-2">
                  <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-primary"></div>
                  <span>Loading logs...</span>
                </div>
              </div>
            ) : logs ? (
              <pre className="text-white/80 whitespace-pre-wrap break-words">{logs}</pre>
            ) : (
              <div className="text-center py-8 text-text-secondary">No logs available</div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default ServiceDetail;
