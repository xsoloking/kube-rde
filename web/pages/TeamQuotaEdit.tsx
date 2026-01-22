import React, { useState, useEffect } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import {
  teamsApi,
  resourceConfigApi,
  Team,
  TeamQuotaWithUsage,
  ResourceConfig,
  GPUTypeConfig,
  StorageClassConfig,
} from '../services/api';

const TeamQuotaEdit: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const teamId = parseInt(id || '0', 10);

  const [team, setTeam] = useState<Team | null>(null);
  const [quotas, setQuotas] = useState<TeamQuotaWithUsage[]>([]);
  const [resourceConfig, setResourceConfig] = useState<ResourceConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Local quota values for editing
  const [quotaValues, setQuotaValues] = useState<{ [key: number]: number }>({});

  useEffect(() => {
    if (teamId) {
      loadData();
    }
  }, [teamId]);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);

      const [teamData, quotaData, configData] = await Promise.all([
        teamsApi.get(teamId),
        teamsApi.getQuota(teamId),
        resourceConfigApi.get(),
      ]);

      setTeam(teamData);
      setQuotas(quotaData);
      setResourceConfig(configData);

      // Initialize quota values from existing quotas
      const values: { [key: number]: number } = {};
      quotaData.forEach((q) => {
        values[q.resource_config_id] = q.quota;
      });
      setQuotaValues(values);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load data';
      setError(message);
      console.error('Failed to load team quota:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    try {
      setSaving(true);

      const quotaUpdates = Object.entries(quotaValues).map(([resourceConfigId, quota]) => ({
        resource_config_id: parseInt(resourceConfigId, 10),
        quota: quota,
      }));

      await teamsApi.updateQuota(teamId, { quotas: quotaUpdates });
      alert('Team quota saved successfully');
      navigate('/teams');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to save quota';
      alert('Error: ' + message);
    } finally {
      setSaving(false);
    }
  };

  // Parse resource config to get available resource types
  const getGPUTypes = (): GPUTypeConfig[] => {
    if (!resourceConfig?.gpu_types) return [];
    if (typeof resourceConfig.gpu_types === 'string') {
      try {
        return JSON.parse(resourceConfig.gpu_types);
      } catch {
        return [];
      }
    }
    return resourceConfig.gpu_types;
  };

  const getStorageClasses = (): StorageClassConfig[] => {
    if (!resourceConfig?.storage_classes) return [];
    if (typeof resourceConfig.storage_classes === 'string') {
      try {
        return JSON.parse(resourceConfig.storage_classes);
      } catch {
        return [];
      }
    }
    return resourceConfig.storage_classes;
  };

  if (loading) {
    return (
      <div className="p-8 lg:p-12 max-w-[1400px] mx-auto flex items-center justify-center min-h-[400px]">
        <div className="flex items-center gap-3">
          <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary"></div>
          <span className="text-text-secondary">Loading team quota...</span>
        </div>
      </div>
    );
  }

  if (error || !team) {
    return (
      <div className="p-8 lg:p-12 max-w-[1400px] mx-auto">
        <div className="bg-red-500/10 border border-red-500/20 rounded-xl p-6 text-center">
          <span className="material-symbols-outlined text-red-500 text-[48px] block mb-2">
            error_outline
          </span>
          <p className="text-red-500 mb-4">{error || 'Team not found'}</p>
          <Link
            to="/teams"
            className="inline-flex items-center gap-2 px-4 py-2 bg-surface-dark border border-border-dark rounded-xl hover:bg-surface-highlight transition-colors"
          >
            <span className="material-symbols-outlined">arrow_back</span>
            Back to Teams
          </Link>
        </div>
      </div>
    );
  }

  const gpuTypes = getGPUTypes();
  const storageClasses = getStorageClasses();

  return (
    <div className="p-8 lg:p-12 max-w-[1400px] mx-auto space-y-8 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-6">
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-3 text-text-secondary mb-2">
            <Link to="/teams" className="hover:text-white transition-colors">
              Teams
            </Link>
            <span className="material-symbols-outlined text-[16px]">chevron_right</span>
            <span className="text-white">{team.display_name}</span>
            <span className="material-symbols-outlined text-[16px]">chevron_right</span>
            <span className="text-white">Quota</span>
          </div>
          <h2 className="text-4xl font-bold tracking-tight">Team Quota</h2>
          <p className="text-text-secondary max-w-2xl text-lg">
            Set resource quotas for <span className="text-white font-medium">{team.display_name}</span>.
            Resources will be shared among all team members.
          </p>
        </div>
        <div className="flex gap-3">
          <Link
            to="/teams"
            className="flex items-center gap-2 px-6 py-3 bg-background-dark border border-border-dark rounded-xl font-bold hover:bg-surface-highlight transition-all"
          >
            Cancel
          </Link>
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-2 bg-primary hover:bg-primary-dark text-white px-6 py-3 rounded-xl font-bold shadow-xl shadow-primary/20 transition-all active:scale-95 disabled:opacity-50"
          >
            <span className="material-symbols-outlined text-[22px]">save</span>
            <span>{saving ? 'Saving...' : 'Save Quota'}</span>
          </button>
        </div>
      </div>

      {/* Team Info */}
      <div className="bg-surface-dark rounded-xl border border-border-dark p-6">
        <div className="flex items-center gap-4">
          <div className="w-12 h-12 rounded-xl bg-primary/10 flex items-center justify-center">
            <span className="material-symbols-outlined text-primary text-[24px]">groups</span>
          </div>
          <div>
            <h3 className="text-xl font-bold">{team.display_name}</h3>
            <p className="text-text-secondary text-sm">
              Namespace: <code className="px-2 py-0.5 bg-background-dark rounded">{team.namespace}</code>
            </p>
          </div>
        </div>
      </div>

      {/* Basic Resources */}
      <div className="bg-surface-dark rounded-xl border border-border-dark overflow-hidden">
        <div className="p-6 border-b border-border-dark">
          <h3 className="text-xl font-bold flex items-center gap-2">
            <span className="material-symbols-outlined text-primary">memory</span>
            Compute Resources
          </h3>
          <p className="text-text-secondary text-sm mt-1">
            CPU and memory limits for the team
          </p>
        </div>
        <div className="p-6 space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <label className="block text-sm font-bold text-text-secondary mb-2">
                CPU Cores (Total)
              </label>
              <input
                type="number"
                min="0"
                value={quotaValues[1] || resourceConfig?.default_cpu_cores || 0}
                onChange={(e) =>
                  setQuotaValues({ ...quotaValues, 1: parseInt(e.target.value, 10) || 0 })
                }
                className="w-full h-11 px-4 bg-background-dark border border-border-dark rounded-xl focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all text-sm"
              />
              <p className="text-xs text-text-secondary mt-1">
                Maximum CPU cores available to the team
              </p>
            </div>
            <div>
              <label className="block text-sm font-bold text-text-secondary mb-2">
                Memory (GiB)
              </label>
              <input
                type="number"
                min="0"
                value={quotaValues[2] || resourceConfig?.default_memory_gi || 0}
                onChange={(e) =>
                  setQuotaValues({ ...quotaValues, 2: parseInt(e.target.value, 10) || 0 })
                }
                className="w-full h-11 px-4 bg-background-dark border border-border-dark rounded-xl focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all text-sm"
              />
              <p className="text-xs text-text-secondary mt-1">
                Maximum memory in GiB available to the team
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Storage Classes */}
      {storageClasses.length > 0 && (
        <div className="bg-surface-dark rounded-xl border border-border-dark overflow-hidden">
          <div className="p-6 border-b border-border-dark">
            <h3 className="text-xl font-bold flex items-center gap-2">
              <span className="material-symbols-outlined text-blue-500">storage</span>
              Storage Classes
            </h3>
            <p className="text-text-secondary text-sm mt-1">
              Storage quotas per storage class
            </p>
          </div>
          <div className="p-6">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {storageClasses.map((sc, index) => (
                <div
                  key={sc.name}
                  className="p-4 bg-background-dark rounded-xl border border-border-dark"
                >
                  <div className="flex items-center justify-between mb-3">
                    <span className="font-bold text-white">{sc.name}</span>
                    {sc.is_default && (
                      <span className="px-2 py-0.5 bg-primary/10 text-primary text-xs rounded-full">
                        Default
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <input
                      type="number"
                      min="0"
                      value={quotaValues[100 + index] || sc.limit_gi || 0}
                      onChange={(e) =>
                        setQuotaValues({
                          ...quotaValues,
                          [100 + index]: parseInt(e.target.value, 10) || 0,
                        })
                      }
                      className="flex-1 h-10 px-3 bg-surface-dark border border-border-dark rounded-lg focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all text-sm"
                    />
                    <span className="text-text-secondary text-sm">GiB</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* GPU Types */}
      {gpuTypes.length > 0 && (
        <div className="bg-surface-dark rounded-xl border border-border-dark overflow-hidden">
          <div className="p-6 border-b border-border-dark">
            <h3 className="text-xl font-bold flex items-center gap-2">
              <span className="material-symbols-outlined text-purple-500">deployed_code</span>
              GPU Resources
            </h3>
            <p className="text-text-secondary text-sm mt-1">
              GPU quotas per GPU type for AI/ML workloads
            </p>
          </div>
          <div className="p-6">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {gpuTypes.map((gpu, index) => (
                <div
                  key={gpu.model_name}
                  className="p-4 bg-background-dark rounded-xl border border-border-dark"
                >
                  <div className="flex items-center gap-3 mb-3">
                    <div className="w-10 h-10 rounded-lg bg-purple-500/10 flex items-center justify-center">
                      <span className="material-symbols-outlined text-purple-500 text-[20px]">
                        deployed_code
                      </span>
                    </div>
                    <div>
                      <span className="font-bold text-white block">{gpu.model_name}</span>
                      <span className="text-xs text-text-secondary">{gpu.resource_name}</span>
                    </div>
                    {gpu.is_default && (
                      <span className="ml-auto px-2 py-0.5 bg-purple-500/10 text-purple-500 text-xs rounded-full">
                        Default
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <input
                      type="number"
                      min="0"
                      value={quotaValues[200 + index] || gpu.limit || 0}
                      onChange={(e) =>
                        setQuotaValues({
                          ...quotaValues,
                          [200 + index]: parseInt(e.target.value, 10) || 0,
                        })
                      }
                      className="flex-1 h-10 px-3 bg-surface-dark border border-border-dark rounded-lg focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all text-sm"
                    />
                    <span className="text-text-secondary text-sm">GPUs</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Current Quota Summary */}
      {quotas.length > 0 && (
        <div className="bg-surface-dark rounded-xl border border-border-dark overflow-hidden">
          <div className="p-6 border-b border-border-dark">
            <h3 className="text-xl font-bold flex items-center gap-2">
              <span className="material-symbols-outlined text-emerald-500">analytics</span>
              Current Usage
            </h3>
            <p className="text-text-secondary text-sm mt-1">
              Resource usage across all team members
            </p>
          </div>
          <div className="p-6">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              {quotas.map((q) => (
                <div
                  key={q.id}
                  className="p-4 bg-background-dark rounded-xl border border-border-dark"
                >
                  <p className="text-text-secondary text-xs font-bold uppercase tracking-wider mb-2">
                    {q.resource_name || `Resource ${q.resource_config_id}`}
                  </p>
                  <div className="flex items-end gap-1">
                    <span className="text-2xl font-bold">{q.used || 0}</span>
                    <span className="text-text-secondary mb-0.5">/ {q.quota}</span>
                  </div>
                  <div className="mt-2 h-1.5 bg-surface-highlight rounded-full overflow-hidden">
                    <div
                      className={`h-full rounded-full transition-all ${
                        ((q.used || 0) / q.quota) > 0.9
                          ? 'bg-red-500'
                          : ((q.used || 0) / q.quota) > 0.7
                            ? 'bg-amber-500'
                            : 'bg-emerald-500'
                      }`}
                      style={{ width: `${Math.min(((q.used || 0) / q.quota) * 100, 100)}%` }}
                    ></div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default TeamQuotaEdit;
