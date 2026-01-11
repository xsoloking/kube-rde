import React, { useState, useEffect } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import {
  servicesApi,
  agentTemplatesApi,
  AgentTemplate,
  userQuotaApi,
  UserQuota,
  UserGPUQuotaItem,
  UpdateServiceRequest,
} from '../services/api';
import { useAuth } from '../contexts/AuthContext';

const ServiceEdit: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user } = useAuth();

  // Service state
  const [serviceName, setServiceName] = useState('');
  const [workspaceId, setWorkspaceId] = useState('');
  const [agentType, setAgentType] = useState('unknown');

  const [template, setTemplate] = useState<AgentTemplate | null>(null);

  // Quota state
  const [quota, setQuota] = useState<UserQuota | null>(null);

  // GPU quota items from user quota
  const [gpuQuota, setGpuQuota] = useState<UserGPUQuotaItem[]>([]);

  // Form state
  const [startupArgs, setStartupArgs] = useState('');
  const [envVars, setEnvVars] = useState<Record<string, string>>({});
  const [showAdvanced, setShowAdvanced] = useState(false);

  // Resource configuration state
  const [cpuCores, setCpuCores] = useState('4');
  const [memoryGiB, setMemoryGiB] = useState('16');
  const [gpuEnabled, setGpuEnabled] = useState(false);
  const [gpuCount, setGpuCount] = useState(1);
  const [selectedGpuQuota, setSelectedGpuQuota] = useState<UserGPUQuotaItem | null>(null);
  const [ttl, setTtl] = useState('24h');

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load service and configs
  useEffect(() => {
    const loadData = async () => {
      if (!id) return;

      try {
        setLoading(true);
        setError(null);

        // 1. Fetch Service
        const service = await servicesApi.get(id);
        setServiceName(service.name);
        setWorkspaceId(service.workspace_id);
        setAgentType(service.agent_type || 'unknown');

        setStartupArgs(service.startup_args || '');
        setEnvVars((service.env_vars as Record<string, string>) || {});

        // Parse resources if available (assuming API returns these fields, otherwise defaults)
        // Note: The Service interface in frontend might need updates to include these fields if they aren't standard yet
        const s = service;
        if (s.cpu_cores) setCpuCores(s.cpu_cores.toString());
        if (s.memory_gib) setMemoryGiB(s.memory_gib.toString());
        if (s.ttl) setTtl(s.ttl);

        // GPU Logic
        if (s.gpu_count && s.gpu_count > 0) {
          setGpuEnabled(true);
          setGpuCount(s.gpu_count);
        }

        // 2. Fetch Template (to get defaults/icons)
        if (service.template_id) {
          try {
            const tmpl = await agentTemplatesApi.get(service.template_id);
            setTemplate(tmpl);
          } catch (e) {
            console.warn('Failed to load template details', e);
          }
        }

        // 3. Fetch User Quota
        if (user?.id) {
          const q = await userQuotaApi.get(user.id);
          setQuota(q);

          // Load GPU quota from user quota
          if (q.gpu_quota && Array.isArray(q.gpu_quota)) {
            setGpuQuota(q.gpu_quota);
            // Match GPU Type by model name
            if (s.gpu_model) {
              const matched = q.gpu_quota.find((t) => t.model_name === s.gpu_model);
              if (matched) setSelectedGpuQuota(matched);
            } else if (q.gpu_quota.length > 0) {
              setSelectedGpuQuota(q.gpu_quota[0]);
            }
          }
        }
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to load service';
        setError(message);
        console.error('Error loading service edit data:', err);
      } finally {
        setLoading(false);
      }
    };

    loadData();
  }, [id, user?.id]);

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;

    try {
      setSaving(true);
      setError(null);

      // Prepare GPU configuration
      const gpuConfig: Partial<UpdateServiceRequest> = {};
      if (gpuEnabled && selectedGpuQuota) {
        gpuConfig.gpu_count = gpuCount;
        gpuConfig.gpu_model = selectedGpuQuota.model_name;
        // Note: gpu_resource_name and gpu_node_selector will be auto-filled by backend
      } else {
        // Explicitly disable GPU if unchecked
        gpuConfig.gpu_count = 0;
        gpuConfig.gpu_model = '';
      }

      // We need to cast to any because UpdateServiceRequest might be incomplete in current type definition
      const updatePayload: UpdateServiceRequest = {
        startup_args: startupArgs,
        env_vars: envVars,
        cpu_cores: cpuCores,
        memory_gib: memoryGiB,
        ttl: ttl,
        ...gpuConfig,
      };

      await servicesApi.update(id, updatePayload);
      navigate(`/workspaces/${workspaceId}`);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to update service';
      setError(message);
      console.error('Error updating service:', err);
    } finally {
      setSaving(false);
    }
  };

  const getTemplateIcon = (type: string) => {
    switch (type) {
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

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="text-center">
          <div className="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-primary mb-4"></div>
          <p className="text-text-secondary">Loading service...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8 lg:px-12 lg:py-6 max-w-[1000px] mx-auto w-full animate-fade-in space-y-8">
      {/* Breadcrumbs */}
      <nav className="flex items-center gap-3 text-xs font-medium">
        <Link to="/workspaces" className="text-text-secondary hover:text-white transition-colors">
          Workspaces
        </Link>
        <span className="material-symbols-outlined text-[14px] text-text-secondary">
          chevron_right
        </span>
        {workspaceId && (
          <>
            <Link
              to={`/workspaces/${workspaceId}`}
              className="text-text-secondary hover:text-white transition-colors truncate"
            >
              {workspaceId}
            </Link>
            <span className="material-symbols-outlined text-[14px] text-text-secondary">
              chevron_right
            </span>
          </>
        )}
        <span className="text-white">Edit Service</span>
      </nav>

      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
        <div>
          <h1 className="text-4xl font-bold tracking-tight mb-2">Edit Service: {serviceName}</h1>
          <p className="text-text-secondary text-base font-light leading-relaxed">
            Update configuration for your {agentType} service
          </p>
        </div>
        <div className="flex items-center gap-4">
          <button
            onClick={() => navigate(-1)}
            className="px-6 py-2.5 text-text-secondary hover:text-white font-bold text-xs uppercase tracking-widest transition-all"
            disabled={saving}
          >
            Cancel
          </button>
          <button
            onClick={handleUpdate}
            disabled={saving}
            className={`bg-primary text-white px-6 py-2.5 rounded-lg font-bold text-xs uppercase tracking-widest shadow-lg shadow-primary/20 flex items-center gap-2 transition-all active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed`}
          >
            <span className="material-symbols-outlined text-[18px]">
              {saving ? 'hourglass_bottom' : 'save'}
            </span>
            {saving ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </div>

      {error && (
        <div className="rounded-xl bg-red-500/10 border border-red-500/20 p-6">
          <p className="text-red-500 font-semibold">{error}</p>
        </div>
      )}

      {/* Info Card */}
      <div className="bg-surface-dark rounded-xl p-4 flex items-center gap-4 border border-border-dark">
        <div className="w-10 h-10 rounded-lg bg-primary/20 flex items-center justify-center text-primary flex-shrink-0">
          <span className="material-symbols-outlined text-[22px]">
            {getTemplateIcon(agentType)}
          </span>
        </div>
        <div>
          <h3 className="text-sm font-bold text-white">{template?.name || serviceName}</h3>
          <p className="text-xs text-text-secondary">
            ID: {id} | Agent Type: {agentType}
          </p>
        </div>
      </div>

      <form onSubmit={handleUpdate} className="space-y-6">
        {/* Resource Configuration Section */}
        <section className="bg-surface-dark rounded-2xl border border-border-dark p-8 shadow-xl">
          <div className="flex items-center gap-3 mb-8 border-b border-border-dark pb-5">
            <div className="w-8 h-8 rounded-lg bg-primary/20 flex items-center justify-center text-primary">
              <span className="material-symbols-outlined text-[20px]">memory</span>
            </div>
            <h3 className="text-base font-bold text-white">Compute Resources</h3>
          </div>

          <div className="space-y-8">
            {/* CPU and Memory Sliders */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
              {/* CPU Cores */}
              <div className="flex flex-col gap-4">
                <div className="flex justify-between items-end">
                  <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary flex items-center gap-2">
                    <span className="material-symbols-outlined text-[16px] text-primary">
                      memory
                    </span>
                    CPU Cores
                  </label>
                  <span className="text-xl font-bold text-primary font-mono">
                    {cpuCores}{' '}
                    <span className="text-[10px] uppercase font-bold text-text-secondary tracking-widest">
                      Cores
                    </span>
                  </span>
                </div>
                {(() => {
                  const maxCpu = quota?.cpu_cores || 16;
                  const cpuStep = maxCpu <= 2 ? 0.1 : maxCpu <= 4 ? 0.5 : 1;
                  const minCpu = cpuStep;
                  const midCpu = (minCpu + maxCpu) / 2;
                  return (
                    <>
                      <input
                        className="w-full accent-primary"
                        type="range"
                        min={minCpu}
                        max={maxCpu}
                        step={cpuStep}
                        value={cpuCores}
                        onChange={(e) => setCpuCores(e.target.value)}
                        disabled={saving}
                      />
                      <div className="flex justify-between text-[10px] text-text-secondary/60 font-mono font-bold">
                        <span>{minCpu}</span>
                        <span>{Number.isInteger(midCpu) ? midCpu : midCpu.toFixed(1)}</span>
                        <span>{maxCpu}</span>
                      </div>
                      {quota && (
                        <p className="text-[10px] text-primary/80 text-right">
                          Quota: {quota.cpu_cores} Cores
                        </p>
                      )}
                    </>
                  );
                })()}
              </div>

              {/* Memory */}
              <div className="flex flex-col gap-4">
                <div className="flex justify-between items-end">
                  <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary flex items-center gap-2">
                    <span className="material-symbols-outlined text-[16px] text-primary">
                      storage
                    </span>
                    Memory (RAM)
                  </label>
                  <span className="text-xl font-bold text-primary font-mono">
                    {memoryGiB}{' '}
                    <span className="text-[10px] uppercase font-bold text-text-secondary tracking-widest">
                      GiB
                    </span>
                  </span>
                </div>
                {(() => {
                  const maxMem = quota?.memory_gi || 64;
                  const minMem = 1;
                  const midMem = (minMem + maxMem) / 2;
                  return (
                    <>
                      <input
                        className="w-full accent-primary"
                        type="range"
                        min={minMem}
                        max={maxMem}
                        step="1"
                        value={memoryGiB}
                        onChange={(e) => setMemoryGiB(e.target.value)}
                        disabled={saving}
                      />
                      <div className="flex justify-between text-[10px] text-text-secondary/60 font-mono font-bold">
                        <span>{minMem}Gi</span>
                        <span>{Number.isInteger(midMem) ? midMem : midMem.toFixed(1)}Gi</span>
                        <span>{maxMem}Gi</span>
                      </div>
                      {quota && (
                        <p className="text-[10px] text-primary/80 text-right">
                          Quota: {quota.memory_gi}Gi
                        </p>
                      )}
                    </>
                  );
                })()}
              </div>
            </div>

            {/* GPU Configuration */}
            <div className="border-t border-border-dark pt-8">
              <div className="flex items-center justify-between mb-6">
                <div className="flex items-center gap-4">
                  <div className="bg-primary/20 p-3 rounded-xl text-primary">
                    <span className="material-symbols-outlined text-xl">developer_board</span>
                  </div>
                  <div>
                    <h4 className="font-bold text-base text-white">Enable GPU Acceleration</h4>
                    <p className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                      Required for complex model training
                    </p>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => setGpuEnabled(!gpuEnabled)}
                  disabled={saving}
                  className={`w-12 h-6 rounded-full transition-all duration-300 relative ${gpuEnabled ? 'bg-primary' : 'bg-background-dark'} disabled:opacity-50`}
                >
                  <div
                    className={`absolute top-1 size-4 rounded-full bg-white transition-all duration-300 shadow-lg ${gpuEnabled ? 'left-7' : 'left-1'}`}
                  ></div>
                </button>
              </div>

              {gpuEnabled && (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6 animate-fade-in">
                  <div className="flex flex-col gap-2">
                    <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                      GPU Model
                    </label>
                    <select
                      className="w-full bg-background-dark border border-border-dark rounded-xl px-4 py-3 text-white focus:ring-1 focus:ring-primary focus:border-primary transition-all text-sm"
                      value={selectedGpuQuota?.model_name || ''}
                      onChange={(e) => {
                        const selected = gpuQuota.find((t) => t.model_name === e.target.value);
                        setSelectedGpuQuota(selected || null);
                      }}
                      disabled={saving || gpuQuota.length === 0}
                    >
                      {gpuQuota.length === 0 ? (
                        <option value="">No GPU quota available</option>
                      ) : (
                        gpuQuota.map((gpu) => (
                          <option key={gpu.model_name} value={gpu.model_name}>
                            {gpu.model_name}
                          </option>
                        ))
                      )}
                    </select>
                  </div>
                  <div className="flex flex-col gap-2">
                    <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                      GPU Count
                    </label>
                    <div className="flex items-center h-12 border border-border-dark rounded-xl bg-background-dark px-2 w-36">
                      <button
                        type="button"
                        onClick={() => setGpuCount(Math.max(1, gpuCount - 1))}
                        disabled={saving || gpuCount <= 1}
                        className="size-8 flex items-center justify-center text-text-secondary hover:text-white hover:bg-white/5 rounded-lg transition-all disabled:opacity-30"
                      >
                        <span className="material-symbols-outlined text-sm">remove</span>
                      </button>
                      <input
                        className="flex-1 bg-transparent text-center text-white font-mono font-bold border-none focus:ring-0 text-base"
                        type="text"
                        readOnly
                        value={gpuCount}
                      />
                      <button
                        type="button"
                        onClick={() => {
                          const limit = selectedGpuQuota?.limit || 8;
                          setGpuCount(Math.min(limit, gpuCount + 1));
                        }}
                        disabled={
                          saving || !selectedGpuQuota || gpuCount >= (selectedGpuQuota?.limit || 0)
                        }
                        className="size-8 flex items-center justify-center text-text-secondary hover:text-white hover:bg-white/5 rounded-lg transition-all disabled:opacity-30"
                      >
                        <span className="material-symbols-outlined text-sm">add</span>
                      </button>
                    </div>
                    {selectedGpuQuota && (
                      <p className="text-[10px] text-text-secondary font-medium">
                        Quota: {selectedGpuQuota.limit} GPUs
                      </p>
                    )}
                  </div>
                </div>
              )}
            </div>

            {/* TTL Configuration */}
            <div className="border-t border-border-dark pt-8">
              <div className="flex items-center gap-4 mb-6">
                <div className="bg-primary/20 p-3 rounded-xl text-primary">
                  <span className="material-symbols-outlined text-xl">schedule</span>
                </div>
                <div className="flex-1">
                  <h4 className="font-bold text-base text-white">Idle Timeout (TTL)</h4>
                  <p className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                    Auto-stop after inactivity period
                  </p>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div className="flex flex-col gap-2">
                  <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                    Timeout Duration
                  </label>
                  <input
                    className="w-full bg-background-dark border border-border-dark rounded-xl px-4 py-3 text-white focus:ring-1 focus:ring-primary focus:border-primary transition-all text-sm font-mono"
                    type="text"
                    value={ttl}
                    onChange={(e) => setTtl(e.target.value)}
                    placeholder="24h"
                    disabled={saving}
                  />
                  <p className="text-[10px] text-text-secondary font-medium">
                    Examples:{' '}
                    <code className="px-1.5 py-0.5 bg-background-dark rounded text-[10px]">
                      24h
                    </code>
                    ,{' '}
                    <code className="px-1.5 py-0.5 bg-background-dark rounded text-[10px]">8h</code>
                    ,{' '}
                    <code className="px-1.5 py-0.5 bg-background-dark rounded text-[10px]">
                      30m
                    </code>
                    ,{' '}
                    <code className="px-1.5 py-0.5 bg-background-dark rounded text-[10px]">0</code>{' '}
                    (disabled)
                  </p>
                </div>
                <div className="flex flex-col gap-3">
                  <div className="text-[10px] font-bold uppercase tracking-widest text-text-secondary mb-2">
                    Quick Presets
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    <button
                      type="button"
                      onClick={() => setTtl('8h')}
                      className={`px-3 py-2 rounded-lg text-xs font-bold transition-all ${ttl === '8h' ? 'bg-primary text-white' : 'bg-background-dark text-text-secondary hover:text-white'}`}
                      disabled={saving}
                    >
                      8 Hours
                    </button>
                    <button
                      type="button"
                      onClick={() => setTtl('24h')}
                      className={`px-3 py-2 rounded-lg text-xs font-bold transition-all ${ttl === '24h' ? 'bg-primary text-white' : 'bg-background-dark text-text-secondary hover:text-white'}`}
                      disabled={saving}
                    >
                      24 Hours
                    </button>
                    <button
                      type="button"
                      onClick={() => setTtl('72h')}
                      className={`px-3 py-2 rounded-lg text-xs font-bold transition-all ${ttl === '72h' ? 'bg-primary text-white' : 'bg-background-dark text-text-secondary hover:text-white'}`}
                      disabled={saving}
                    >
                      3 Days
                    </button>
                    <button
                      type="button"
                      onClick={() => setTtl('0')}
                      className={`px-3 py-2 rounded-lg text-xs font-bold transition-all ${ttl === '0' ? 'bg-primary text-white' : 'bg-background-dark text-text-secondary hover:text-white'}`}
                      disabled={saving}
                    >
                      Disabled
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Advanced Options */}
        <section className="bg-surface-dark rounded-2xl border border-border-dark overflow-hidden shadow-xl">
          <button
            type="button"
            onClick={() => setShowAdvanced(!showAdvanced)}
            className="w-full flex items-center gap-3 p-6 border-b border-border-dark hover:bg-surface-dark/50 transition-all"
            disabled={saving}
          >
            <div className="w-8 h-8 rounded-lg bg-primary/20 flex items-center justify-center text-primary">
              <span className="material-symbols-outlined text-[20px]">settings</span>
            </div>
            <h3 className="text-base font-bold text-white flex-1 text-left">
              Advanced Options (Optional)
            </h3>
            <span
              className={`material-symbols-outlined transition-transform ${showAdvanced ? 'rotate-180' : ''}`}
            >
              expand_more
            </span>
          </button>

          {showAdvanced && (
            <div className="p-6 space-y-6">
              <div className="flex flex-col gap-2">
                <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Startup Arguments
                </label>
                <textarea
                  className="w-full bg-background-dark border border-border-dark rounded-xl p-4 text-white focus:ring-1 focus:ring-primary focus:border-primary placeholder:text-text-secondary/30 transition-all text-sm font-mono"
                  placeholder="(none)"
                  rows={3}
                  value={startupArgs}
                  onChange={(e) => setStartupArgs(e.target.value)}
                  disabled={saving}
                />
                <p className="text-[10px] text-text-secondary font-medium">
                  Override startup arguments
                </p>
              </div>

              <div className="flex flex-col gap-2">
                <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Environment Variables (JSON)
                </label>
                <textarea
                  className="w-full bg-background-dark border border-border-dark rounded-xl p-4 text-white focus:ring-1 focus:ring-primary focus:border-primary placeholder:text-text-secondary/30 transition-all text-sm font-mono"
                  placeholder='e.g., {"KEY": "value"}'
                  rows={3}
                  value={JSON.stringify(envVars, null, 2)}
                  onChange={(e) => {
                    try {
                      setEnvVars(JSON.parse(e.target.value));
                    } catch {
                      // Ignore JSON parse errors while typing
                    }
                  }}
                  disabled={saving}
                />
                <p className="text-[10px] text-text-secondary font-medium">
                  Custom environment variables
                </p>
              </div>
            </div>
          )}
        </section>

        <div className="bg-background-dark/50 rounded-xl p-4 flex items-center gap-4 border border-border-dark">
          <span className="material-symbols-outlined text-amber-400 text-xl icon-fill">
            warning
          </span>
          <p className="text-xs text-text-secondary leading-relaxed">
            Updating configuration will restart the service. Current connection might be lost.
          </p>
        </div>
      </form>
    </div>
  );
};

export default ServiceEdit;
