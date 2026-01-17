import React, { useState, useEffect } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import {
  servicesApi,
  agentTemplatesApi,
  AgentTemplate,
  sshKeysApi,
  userQuotaApi,
  UserQuota,
  UserGPUQuotaItem,
  CreateServiceRequest,
} from '../services/api';
import { useAuth } from '../contexts/AuthContext';

const ServiceCreate: React.FC = () => {
  const { workspaceId } = useParams();
  const navigate = useNavigate();
  const { user, refreshUser } = useAuth();

  // Template selection state
  const [templates, setTemplates] = useState<AgentTemplate[]>([]);
  const [selectedTemplate, setSelectedTemplate] = useState<AgentTemplate | null>(null);
  const [templatesLoading, setTemplatesLoading] = useState(true);

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

  // SSH Key management state
  const [showSshKeyForm, setShowSshKeyForm] = useState(false);
  const [sshKeyName, setSshKeyName] = useState('');
  const [sshKeyPublic, setSshKeyPublic] = useState('');
  const [addingSshKey, setAddingSshKey] = useState(false);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load templates and GPU types on mount
  useEffect(() => {
    const loadTemplates = async () => {
      try {
        const data = await agentTemplatesApi.list();
        setTemplates(data);
      } catch (err) {
        console.error('Failed to load templates:', err);
        setError('Failed to load agent templates');
      } finally {
        setTemplatesLoading(false);
      }
    };

    const loadQuota = async () => {
      if (user?.id) {
        try {
          const q = await userQuotaApi.get(user.id);
          setQuota(q);

          // Load GPU quota from user quota
          if (q.gpu_quota && Array.isArray(q.gpu_quota)) {
            setGpuQuota(q.gpu_quota);
            // Set default GPU quota if available
            if (q.gpu_quota.length > 0) {
              setSelectedGpuQuota(q.gpu_quota[0]);
            }
          }

          // Clamp initial values if they exceed quota
          if (q) {
            setCpuCores((prev) => {
              const val = parseFloat(prev);
              return val > q.cpu_cores ? q.cpu_cores.toString() : prev;
            });
            setMemoryGiB((prev) => {
              const val = parseInt(prev);
              return val > q.memory_gi ? q.memory_gi.toString() : prev;
            });
          }
        } catch (err) {
          console.error('Failed to load quota:', err);
        }
      }
    };

    loadTemplates();
    loadQuota();
  }, [user?.id]);

  // Reset form when template is selected
  useEffect(() => {
    if (selectedTemplate) {
      setStartupArgs('');
      setEnvVars({});
    }
  }, [selectedTemplate]);

  const handleAddSshKey = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!sshKeyName.trim()) {
      setError('SSH key name is required');
      return;
    }

    if (!sshKeyPublic.trim()) {
      setError('SSH public key is required');
      return;
    }

    if (!user?.id) {
      setError('User information not available');
      return;
    }

    try {
      setAddingSshKey(true);
      setError(null);
      await sshKeysApi.create(user.id, {
        name: sshKeyName.trim(),
        public_key: sshKeyPublic.trim(),
      });

      // Refresh user data to get updated SSH keys
      await refreshUser();

      // Reset form and hide it
      setSshKeyName('');
      setSshKeyPublic('');
      setShowSshKeyForm(false);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to add SSH key';
      setError(message);
      console.error('Error adding SSH key:', err);
    } finally {
      setAddingSshKey(false);
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!selectedTemplate) {
      setError('Please select an agent template');
      return;
    }

    if (!workspaceId) {
      setError('Workspace ID is missing');
      return;
    }

    try {
      setLoading(true);
      setError(null);

      // Prepare GPU configuration
      const gpuConfig: Partial<CreateServiceRequest> = {};
      if (gpuEnabled && selectedGpuQuota) {
        gpuConfig.gpu_count = gpuCount;
        gpuConfig.gpu_model = selectedGpuQuota.model_name;
        // Note: gpu_resource_name and gpu_node_selector will be auto-filled by backend
      }

      // We need to cast to any because CreateServiceRequest might be incomplete in current type definition
      const payload: CreateServiceRequest = {
        name: selectedTemplate.name,
        workspace_id: workspaceId,
        template_id: selectedTemplate.id,
        startup_args: startupArgs || undefined,
        env_vars: Object.keys(envVars).length > 0 ? envVars : undefined,
        cpu_cores: cpuCores,
        memory_gib: memoryGiB,
        ttl: ttl,
        ...gpuConfig,
      };

      await servicesApi.create(workspaceId, payload);
      navigate(`/workspaces/${workspaceId}`);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create service';
      setError(message);
      console.error('Error creating service:', err);
    } finally {
      setLoading(false);
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
        <span className="text-white">New Service</span>
      </nav>

      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
        <div>
          <h1 className="text-4xl font-bold tracking-tight mb-2">Create New Service</h1>
          <p className="text-text-secondary text-base font-light leading-relaxed">
            {selectedTemplate
              ? selectedTemplate.agent_type?.toLowerCase().trim() === 'ssh' &&
                (!user?.ssh_keys || user.ssh_keys.length === 0)
                ? 'Add SSH key to continue'
                : `Configure your ${selectedTemplate.name}`
              : 'Choose an agent type to get started'}
          </p>
        </div>
        <div className="flex items-center gap-4">
          <button
            onClick={() => (selectedTemplate ? setSelectedTemplate(null) : navigate(-1))}
            className="px-6 py-2.5 text-text-secondary hover:text-white font-bold text-xs uppercase tracking-widest transition-all"
            disabled={loading}
          >
            {selectedTemplate ? 'Back' : 'Cancel'}
          </button>
          {selectedTemplate && (
            <button
              onClick={handleCreate}
              disabled={loading}
              className={`bg-primary text-white px-6 py-2.5 rounded-lg font-bold text-xs uppercase tracking-widest shadow-lg shadow-primary/20 flex items-center gap-2 transition-all active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed`}
            >
              <span className="material-symbols-outlined text-[18px]">
                {loading ? 'hourglass_bottom' : 'add_circle'}
              </span>
              {loading ? 'Creating...' : 'Create Service'}
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="rounded-xl bg-red-500/10 border border-red-500/20 p-6">
          <p className="text-red-500 font-semibold">{error}</p>
        </div>
      )}

      {/* Template Selection */}
      {!selectedTemplate ? (
        <section className="space-y-6">
          <div className="flex items-center gap-3 mb-6">
            <div className="w-8 h-8 rounded-lg bg-primary/20 flex items-center justify-center text-primary">
              <span className="material-symbols-outlined text-[20px]">category</span>
            </div>
            <h2 className="text-lg font-bold text-white">Select Agent Type</h2>
          </div>

          {templatesLoading ? (
            <div className="text-center py-12">
              <p className="text-text-secondary">Loading templates...</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {templates.map((template) => (
                <button
                  key={template.id}
                  onClick={() => setSelectedTemplate(template)}
                  disabled={loading}
                  className="bg-surface-dark hover:bg-surface-dark/80 border border-border-dark hover:border-primary/50 rounded-2xl p-6 text-left transition-all active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed shadow-lg hover:shadow-primary/10"
                >
                  <div className="flex items-start gap-4">
                    <div className="w-10 h-10 rounded-lg bg-primary/20 flex items-center justify-center text-primary flex-shrink-0">
                      <span className="material-symbols-outlined text-[22px]">
                        {getTemplateIcon(template.agent_type)}
                      </span>
                    </div>
                    <div className="flex-1">
                      <h3 className="text-base font-bold text-white mb-1">{template.name}</h3>
                      <p className="text-xs text-text-secondary line-clamp-2">
                        {template.description || 'No description'}
                      </p>
                      <div className="mt-3 flex gap-2 flex-wrap">
                        <span className="inline-block px-2 py-1 rounded bg-primary/20 text-primary text-[10px] font-bold uppercase">
                          {template.agent_type}
                        </span>
                        <span className="inline-block px-2 py-1 rounded bg-blue-500/20 text-blue-400 text-[10px] font-bold uppercase">
                          Port {template.default_external_port}
                        </span>
                      </div>
                    </div>
                  </div>
                </button>
              ))}
            </div>
          )}
        </section>
      ) : (
        /* Configuration Form */
        <form onSubmit={handleCreate} className="space-y-6">
          {/* SSH Key Warning for SSH Services */}
          {selectedTemplate.agent_type?.toLowerCase().trim() === 'ssh' &&
            (!user?.ssh_keys || user.ssh_keys.length === 0) && (
              <section className="bg-amber-500/10 rounded-2xl border border-amber-500/30 p-6 shadow-xl animate-fade-in">
                <div className="flex items-start gap-4">
                  <div className="size-10 rounded-xl bg-amber-500/20 flex items-center justify-center flex-shrink-0">
                    <span className="material-symbols-outlined text-[24px] text-amber-500">
                      warning
                    </span>
                  </div>
                  <div className="flex-1">
                    <h3 className="text-lg font-bold text-amber-500 mb-2">SSH Key Required</h3>
                    <p className="text-amber-500/80 text-sm mb-4 leading-relaxed">
                      You need to add at least one SSH public key to connect to SSH services.
                      Without it, you won't be able to authenticate.
                    </p>

                    {!showSshKeyForm ? (
                      <button
                        type="button"
                        onClick={() => setShowSshKeyForm(true)}
                        className="px-5 py-2.5 bg-amber-500 hover:bg-amber-600 text-white rounded-lg font-bold text-xs uppercase tracking-widest transition-all flex items-center gap-2 shadow-lg shadow-amber-500/20"
                      >
                        <span className="material-symbols-outlined text-[18px]">key</span>
                        Add SSH Key Now
                      </button>
                    ) : (
                      <div className="bg-background-dark/50 rounded-xl border border-amber-500/20 p-6 space-y-4 animate-fade-in">
                        <div className="flex items-center justify-between mb-4 pb-3 border-b border-amber-500/20">
                          <h4 className="text-sm font-bold text-white">Add Your SSH Public Key</h4>
                          <button
                            type="button"
                            onClick={() => {
                              setShowSshKeyForm(false);
                              setSshKeyName('');
                              setSshKeyPublic('');
                              setError(null);
                            }}
                            className="text-text-secondary hover:text-white transition-colors"
                          >
                            <span className="material-symbols-outlined text-[20px]">close</span>
                          </button>
                        </div>

                        <div className="flex flex-col gap-2">
                          <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                            Key Name <span className="text-red-500">*</span>
                          </label>
                          <input
                            type="text"
                            value={sshKeyName}
                            onChange={(e) => setSshKeyName(e.target.value)}
                            placeholder="e.g., My MacBook Pro"
                            className="w-full bg-background-dark border border-border-dark rounded-lg h-11 px-4 text-white focus:ring-1 focus:ring-amber-500 focus:border-amber-500 placeholder:text-text-secondary/30 transition-all text-sm"
                            disabled={addingSshKey}
                          />
                        </div>

                        <div className="flex flex-col gap-2">
                          <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                            SSH Public Key <span className="text-red-500">*</span>
                          </label>
                          <textarea
                            value={sshKeyPublic}
                            onChange={(e) => setSshKeyPublic(e.target.value)}
                            placeholder="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ... user@hostname"
                            rows={4}
                            className="w-full bg-background-dark border border-border-dark rounded-lg p-3 text-white focus:ring-1 focus:ring-amber-500 focus:border-amber-500 placeholder:text-text-secondary/30 transition-all text-xs font-mono resize-none"
                            disabled={addingSshKey}
                          />
                          <p className="text-[10px] text-text-secondary font-medium">
                            Paste your public key from{' '}
                            <code className="px-1.5 py-0.5 bg-background-dark rounded text-[10px]">
                              ~/.ssh/id_rsa.pub
                            </code>{' '}
                            or generate one with{' '}
                            <code className="px-1.5 py-0.5 bg-background-dark rounded text-[10px]">
                              ssh-keygen
                            </code>
                          </p>
                        </div>

                        <div className="flex items-center gap-3 pt-2">
                          <button
                            type="button"
                            onClick={handleAddSshKey}
                            disabled={addingSshKey || !sshKeyName.trim() || !sshKeyPublic.trim()}
                            className="px-5 py-2.5 bg-amber-500 hover:bg-amber-600 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg font-bold text-xs uppercase tracking-widest transition-all flex items-center gap-2 shadow-lg shadow-amber-500/20"
                          >
                            {addingSshKey ? (
                              <>
                                <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
                                Adding...
                              </>
                            ) : (
                              <>
                                <span className="material-symbols-outlined text-[16px]">check</span>
                                Add Key
                              </>
                            )}
                          </button>
                          <button
                            type="button"
                            onClick={() => {
                              setShowSshKeyForm(false);
                              setSshKeyName('');
                              setSshKeyPublic('');
                              setError(null);
                            }}
                            disabled={addingSshKey}
                            className="px-5 py-2.5 text-text-secondary hover:text-white font-bold text-xs uppercase tracking-widest transition-all disabled:opacity-50"
                          >
                            Cancel
                          </button>
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              </section>
            )}

          {/* SSH Key Success Message */}
          {selectedTemplate.agent_type?.toLowerCase().trim() === 'ssh' &&
            user?.ssh_keys &&
            user.ssh_keys.length > 0 && (
              <section className="bg-emerald-500/10 rounded-2xl border border-emerald-500/30 p-5 shadow-xl animate-fade-in">
                <div className="flex items-center gap-3">
                  <div className="size-8 rounded-lg bg-emerald-500/20 flex items-center justify-center">
                    <span className="material-symbols-outlined text-[20px] text-emerald-500">
                      check_circle
                    </span>
                  </div>
                  <div className="flex-1">
                    <p className="text-sm font-semibold text-emerald-500">
                      SSH Key Configured ({user.ssh_keys.length} key
                      {user.ssh_keys.length > 1 ? 's' : ''})
                    </p>
                    <p className="text-xs text-emerald-500/70 mt-0.5">
                      You're ready to connect to SSH services
                    </p>
                  </div>
                </div>
              </section>
            )}

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
                          disabled={loading}
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
                          disabled={loading}
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
                    disabled={loading}
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
                          const modelName = e.target.value;
                          const matched = gpuQuota.find((t) => t.model_name === modelName);
                          setSelectedGpuQuota(matched || null);
                        }}
                        disabled={loading || gpuQuota.length === 0}
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
                          disabled={loading || gpuCount <= 1}
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
                            loading ||
                            !selectedGpuQuota ||
                            gpuCount >= (selectedGpuQuota?.limit || 0)
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
                      disabled={loading}
                    />
                    <p className="text-[10px] text-text-secondary font-medium">
                      Examples:{' '}
                      <code className="px-1.5 py-0.5 bg-background-dark rounded text-[10px]">
                        24h
                      </code>
                      ,{' '}
                      <code className="px-1.5 py-0.5 bg-background-dark rounded text-[10px]">
                        8h
                      </code>
                      ,{' '}
                      <code className="px-1.5 py-0.5 bg-background-dark rounded text-[10px]">
                        30m
                      </code>
                      ,{' '}
                      <code className="px-1.5 py-0.5 bg-background-dark rounded text-[10px]">
                        0
                      </code>{' '}
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
                        disabled={loading}
                      >
                        8 Hours
                      </button>
                      <button
                        type="button"
                        onClick={() => setTtl('24h')}
                        className={`px-3 py-2 rounded-lg text-xs font-bold transition-all ${ttl === '24h' ? 'bg-primary text-white' : 'bg-background-dark text-text-secondary hover:text-white'}`}
                        disabled={loading}
                      >
                        24 Hours
                      </button>
                      <button
                        type="button"
                        onClick={() => setTtl('72h')}
                        className={`px-3 py-2 rounded-lg text-xs font-bold transition-all ${ttl === '72h' ? 'bg-primary text-white' : 'bg-background-dark text-text-secondary hover:text-white'}`}
                        disabled={loading}
                      >
                        3 Days
                      </button>
                      <button
                        type="button"
                        onClick={() => setTtl('0')}
                        className={`px-3 py-2 rounded-lg text-xs font-bold transition-all ${ttl === '0' ? 'bg-primary text-white' : 'bg-background-dark text-text-secondary hover:text-white'}`}
                        disabled={loading}
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
              disabled={loading}
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
                    placeholder={selectedTemplate.startup_args || '(none)'}
                    rows={3}
                    value={startupArgs}
                    onChange={(e) => setStartupArgs(e.target.value)}
                    disabled={loading}
                  />
                  <p className="text-[10px] text-text-secondary font-medium">
                    Override startup arguments (leave empty to use template defaults)
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
                    disabled={loading}
                  />
                  <p className="text-[10px] text-text-secondary font-medium">
                    Custom environment variables will override template defaults
                  </p>
                </div>
              </div>
            )}
          </section>

          <div className="bg-background-dark/50 rounded-xl p-4 flex items-center gap-4 border border-border-dark">
            <span className="material-symbols-outlined text-blue-400 text-xl icon-fill">info</span>
            <p className="text-xs text-text-secondary leading-relaxed">
              Your service will be created with the <strong>{selectedTemplate.name}</strong>{' '}
              configuration. Once created, it will be accessible through the workspace proxy.
            </p>
          </div>
        </form>
      )}
    </div>
  );
};

export default ServiceCreate;
