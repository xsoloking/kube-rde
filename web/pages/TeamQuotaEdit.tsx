import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { teamsApi, Team, TeamQuotaItem } from '../services/api';

const TeamQuotaEdit: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const teamId = parseInt(id || '0', 10);

  const [team, setTeam] = useState<Team | null>(null);
  const [quotas, setQuotas] = useState<TeamQuotaItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Local quota values for editing
  const [quotaValues, setQuotaValues] = useState<{ [key: number]: number }>({});

  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const [teamData, quotaData] = await Promise.all([
        teamsApi.get(teamId),
        teamsApi.getQuota(teamId),
      ]);

      setTeam(teamData);
      setQuotas(quotaData || []);

      // Initialize quota values from existing quotas
      const values: { [key: number]: number } = {};
      (quotaData || []).forEach((q: TeamQuotaItem) => {
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
  }, [teamId]);

  useEffect(() => {
    if (teamId) {
      loadData();
    }
  }, [teamId, loadData]);

  const handleSave = async () => {
    try {
      setSaving(true);

      // Build quota updates with resource type and name from the original quotas
      const quotaUpdates = quotas.map((q) => ({
        resource_config_id: q.resource_config_id,
        resource_type: q.resource_type,
        resource_name: q.resource_name,
        quota: quotaValues[q.resource_config_id] || 0,
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

  // Group quotas by type
  const computeQuotas = quotas.filter(
    (q) => q.resource_type === 'cpu' || q.resource_type === 'memory',
  );
  const storageQuotas = quotas.filter((q) => q.resource_type === 'storage');
  const gpuQuotas = quotas.filter((q) => q.resource_type === 'gpu');

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

  return (
    <div className="p-8 lg:p-12 max-w-[1400px] mx-auto space-y-8 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-6">
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-3 text-text-secondary mb-2">
            <Link to="/teams" className="hover:text-text-foreground transition-colors">
              Teams
            </Link>
            <span className="material-symbols-outlined text-[16px]">chevron_right</span>
            <span className="text-text-foreground">{team.display_name}</span>
            <span className="material-symbols-outlined text-[16px]">chevron_right</span>
            <span className="text-text-foreground">Quota</span>
          </div>
          <h2 className="text-4xl font-bold tracking-tight">Team Quota</h2>
          <p className="text-text-secondary max-w-2xl text-lg">
            Set resource quotas for{' '}
            <span className="text-text-foreground font-medium">{team.display_name}</span>. Resources
            will be shared among all team members.
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
              Namespace:{' '}
              <code className="px-2 py-0.5 bg-background-dark rounded">{team.namespace}</code>
            </p>
          </div>
        </div>
      </div>

      {/* Compute Resources (CPU & Memory) */}
      {computeQuotas.length > 0 && (
        <div className="bg-surface-dark rounded-xl border border-border-dark overflow-hidden">
          <div className="p-6 border-b border-border-dark">
            <h3 className="text-xl font-bold flex items-center gap-2">
              <span className="material-symbols-outlined text-primary">memory</span>
              Compute Resources
            </h3>
            <p className="text-text-secondary text-sm mt-1">CPU and memory limits for the team</p>
          </div>
          <div className="p-6">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {computeQuotas.map((q) => (
                <div key={q.resource_config_id}>
                  <label className="block text-sm font-bold text-text-secondary mb-2">
                    {q.display_name} ({q.unit})
                  </label>
                  <input
                    type="number"
                    min="0"
                    value={quotaValues[q.resource_config_id] || 0}
                    onChange={(e) =>
                      setQuotaValues({
                        ...quotaValues,
                        [q.resource_config_id]: parseInt(e.target.value, 10) || 0,
                      })
                    }
                    className="w-full h-11 px-4 bg-background-dark border border-border-dark rounded-xl focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all text-sm"
                  />
                  <p className="text-xs text-text-secondary mt-1">
                    Maximum {q.display_name.toLowerCase()} available to the team
                  </p>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Storage */}
      {storageQuotas.length > 0 && (
        <div className="bg-surface-dark rounded-xl border border-border-dark overflow-hidden">
          <div className="p-6 border-b border-border-dark">
            <h3 className="text-xl font-bold flex items-center gap-2">
              <span className="material-symbols-outlined text-blue-500">storage</span>
              Storage Resources
            </h3>
            <p className="text-text-secondary text-sm mt-1">Storage quotas for the team</p>
          </div>
          <div className="p-6">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {storageQuotas.map((q) => (
                <div
                  key={q.resource_config_id}
                  className="p-4 bg-background-dark rounded-xl border border-border-dark"
                >
                  <div className="flex items-center justify-between mb-3">
                    <span className="font-bold text-text-foreground">{q.display_name}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <input
                      type="number"
                      min="0"
                      value={quotaValues[q.resource_config_id] || 0}
                      onChange={(e) =>
                        setQuotaValues({
                          ...quotaValues,
                          [q.resource_config_id]: parseInt(e.target.value, 10) || 0,
                        })
                      }
                      className="flex-1 h-10 px-3 bg-surface-dark border border-border-dark rounded-lg focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all text-sm"
                    />
                    <span className="text-text-secondary text-sm">{q.unit}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* GPU Types */}
      {gpuQuotas.length > 0 && (
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
              {gpuQuotas.map((q) => (
                <div
                  key={q.resource_config_id}
                  className="p-4 bg-background-dark rounded-xl border border-border-dark"
                >
                  <div className="flex items-center gap-3 mb-3">
                    <div className="w-10 h-10 rounded-lg bg-purple-500/10 flex items-center justify-center">
                      <span className="material-symbols-outlined text-purple-500 text-[20px]">
                        deployed_code
                      </span>
                    </div>
                    <div>
                      <span className="font-bold text-text-foreground block">{q.display_name}</span>
                      <span className="text-xs text-text-secondary">{q.resource_name}</span>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <input
                      type="number"
                      min="0"
                      value={quotaValues[q.resource_config_id] || 0}
                      onChange={(e) =>
                        setQuotaValues({
                          ...quotaValues,
                          [q.resource_config_id]: parseInt(e.target.value, 10) || 0,
                        })
                      }
                      className="flex-1 h-10 px-3 bg-surface-dark border border-border-dark rounded-lg focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all text-sm"
                    />
                    <span className="text-text-secondary text-sm">{q.unit}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Empty State */}
      {quotas.length === 0 && (
        <div className="bg-surface-dark rounded-xl border border-border-dark p-12 text-center">
          <span className="material-symbols-outlined text-text-secondary text-[64px] block mb-4">
            inventory_2
          </span>
          <h3 className="text-xl font-bold mb-2">No Resource Types Configured</h3>
          <p className="text-text-secondary max-w-md mx-auto">
            Resource types need to be configured in the system settings before team quotas can be
            set.
          </p>
        </div>
      )}
    </div>
  );
};

export default TeamQuotaEdit;
