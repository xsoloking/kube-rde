import React, { useState, useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { workspacesApi, userQuotaApi, UserQuota, StorageClassConfig } from '../services/api';
import { useAuth } from '../contexts/AuthContext';

const WorkspaceCreate: React.FC = () => {
  const navigate = useNavigate();
  const { user } = useAuth();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [storageSize, setStorageSize] = useState('10');
  const [storageClass, setStorageClass] = useState('standard');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [quota, setQuota] = useState<UserQuota | null>(null);
  const [availableStorageClasses, setAvailableStorageClasses] = useState<StorageClassConfig[]>([]);

  useEffect(() => {
    loadQuotaAndConfig();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const loadQuotaAndConfig = async () => {
    try {
      if (!user?.id) return;

      const quotaData = await userQuotaApi.get(user.id);
      setQuota(quotaData);

      // Derive available storage classes from user quota
      let classes: StorageClassConfig[] = [];
      if (quotaData && quotaData.storage_quota) {
        classes = quotaData.storage_quota.map((item) => ({
          name: item.name,
          limit_gi: item.limit_gi,
          is_default: false,
        }));
      }

      setAvailableStorageClasses(classes);

      if (classes.length > 0) {
        const defaultClass =
          classes.find((c) => c.name === 'standard' || c.name === 'default') || classes[0];
        setStorageClass(defaultClass.name);

        const limit = defaultClass.limit_gi;
        if (parseInt(storageSize) > limit) {
          setStorageSize(limit.toString());
        }
      }
    } catch (err) {
      console.error('Failed to load user quota:', err);
      setError('Failed to load user quota');
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!name.trim()) {
      setError('Workspace name is required');
      return;
    }

    const sizeGi = parseInt(storageSize) || 10;

    // Client-side quota validation
    if (quota && quota.storage_quota) {
      const quotaItem = quota.storage_quota.find((item) => item.name === storageClass);
      const limit = quotaItem?.limit_gi || 0;
      if (sizeGi > limit) {
        setError(`Storage size ${sizeGi}Gi exceeds your quota of ${limit}Gi for ${storageClass}`);
        return;
      }
    }

    try {
      setLoading(true);
      setError(null);
      await workspacesApi.create({
        name: name.trim(),
        description: description.trim() || undefined,
        storage_size: `${sizeGi}Gi`,
        storage_class: storageClass,
      });
      navigate('/workspaces');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create workspace';
      setError(message);
      console.error('Error creating workspace:', err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="h-full flex flex-col overflow-hidden">
      <div className="flex-1 overflow-y-auto">
        <div className="p-5 lg:p-6 max-w-[1200px] mx-auto w-full animate-fade-in">
          {/* Compact Header with Breadcrumb */}
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <Link
                to="/workspaces"
                className="text-text-secondary hover:text-white transition-colors text-[10px] font-bold uppercase tracking-widest"
              >
                ← Workspaces
              </Link>
              <span className="text-text-secondary">/</span>
              <h1 className="text-2xl font-bold tracking-tight text-white">New Workspace</h1>
            </div>
            <div className="flex items-center gap-3">
              <button
                onClick={() => navigate(-1)}
                className="px-4 py-2 text-text-secondary hover:text-white font-bold text-[10px] uppercase tracking-widest transition-all"
                disabled={loading}
              >
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={loading || !name.trim()}
                className="bg-primary text-white px-5 py-2 rounded-lg font-bold text-[10px] uppercase tracking-widest shadow-lg shadow-primary/20 flex items-center gap-2 transition-all active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <span className="material-symbols-outlined text-[16px]">
                  {loading ? 'hourglass_bottom' : 'add_circle'}
                </span>
                {loading ? 'Creating...' : 'Create'}
              </button>
            </div>
          </div>

          {error && (
            <div className="rounded-lg bg-red-500/10 border border-red-500/20 p-3 mb-4">
              <p className="text-red-500 text-sm font-semibold">{error}</p>
            </div>
          )}

          <form onSubmit={handleCreate}>
            {/* Unified Section with Grid Layout */}
            <div className="bg-surface-dark rounded-xl border border-border-dark p-5 shadow-xl">
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                {/* Left Column: General Information */}
                <div className="space-y-4">
                  <div className="flex items-center gap-2 pb-3 border-b border-border-dark/50">
                    <span className="material-symbols-outlined text-primary text-[18px]">
                      badge
                    </span>
                    <h3 className="text-sm font-bold text-white">General Information</h3>
                  </div>

                  <div className="space-y-3">
                    <div className="flex flex-col gap-1.5">
                      <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                        Workspace Name <span className="text-red-500">*</span>
                      </label>
                      <input
                        className="w-full bg-background-dark border border-border-dark rounded-lg h-9 px-3 text-white focus:ring-1 focus:ring-primary focus:border-primary placeholder:text-text-secondary/30 transition-all text-sm"
                        placeholder="e.g., production-env"
                        type="text"
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        disabled={loading}
                      />
                    </div>

                    <div className="flex flex-col gap-1.5">
                      <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                        Description <span className="text-slate-500 font-normal">(Optional)</span>
                      </label>
                      <textarea
                        className="w-full bg-background-dark border border-border-dark rounded-lg p-3 text-white focus:ring-1 focus:ring-primary focus:border-primary outline-none placeholder:text-text-secondary/30 text-sm leading-relaxed resize-none"
                        placeholder="Describe the purpose..."
                        rows={3}
                        value={description}
                        onChange={(e) => setDescription(e.target.value)}
                        disabled={loading}
                      ></textarea>
                    </div>
                  </div>
                </div>

                {/* Right Column: Storage Configuration */}
                <div className="space-y-4">
                  <div className="flex items-center gap-2 pb-3 border-b border-border-dark/50">
                    <span className="material-symbols-outlined text-primary text-[18px]">
                      storage
                    </span>
                    <h3 className="text-sm font-bold text-white">Storage Configuration</h3>
                  </div>

                  <div className="space-y-3">
                    <div className="flex flex-col gap-1.5">
                      <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                        Storage Class <span className="text-red-500">*</span>
                      </label>
                      <select
                        className="w-full bg-background-dark border border-border-dark rounded-lg h-9 px-3 text-white focus:ring-1 focus:ring-primary focus:border-primary transition-all text-sm cursor-pointer"
                        value={storageClass}
                        onChange={(e) => {
                          setStorageClass(e.target.value);
                          const quotaItem = quota?.storage_quota?.find(
                            (item) => item.name === e.target.value,
                          );
                          const newLimit = quotaItem?.limit_gi || 100;
                          if (parseInt(storageSize) > newLimit) {
                            setStorageSize(newLimit.toString());
                          }
                        }}
                        disabled={loading || availableStorageClasses.length === 0}
                      >
                        {availableStorageClasses.length === 0 ? (
                          <option value="standard">standard (default)</option>
                        ) : (
                          availableStorageClasses.map((sc) => {
                            const quotaItem = quota?.storage_quota?.find(
                              (item) => item.name === sc.name,
                            );
                            const limit = quotaItem?.limit_gi;
                            return (
                              <option key={sc.name} value={sc.name}>
                                {sc.name} {limit !== undefined ? `(${limit}Gi)` : ''}
                              </option>
                            );
                          })
                        )}
                      </select>
                    </div>

                    <div className="flex flex-col gap-1.5">
                      <div className="flex items-center justify-between">
                        <label className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                          Storage Size <span className="text-red-500">*</span>
                        </label>
                        <span className="text-lg font-bold text-primary font-mono">
                          {storageSize}
                          <span className="text-[10px] text-text-secondary ml-1">Gi</span>
                        </span>
                      </div>
                      <input
                        className="w-full accent-primary h-2"
                        type="range"
                        min="1"
                        max={(() => {
                          const quotaItem = quota?.storage_quota?.find(
                            (item) => item.name === storageClass,
                          );
                          return quotaItem?.limit_gi || 100;
                        })()}
                        step="1"
                        value={storageSize}
                        onChange={(e) => setStorageSize(e.target.value)}
                        disabled={loading}
                      />
                      <div className="flex items-center justify-between text-[10px]">
                        <span className="text-text-secondary">1 Gi</span>
                        <span className="text-primary font-bold">
                          Quota:{' '}
                          {quota?.storage_quota?.find((item) => item.name === storageClass)
                            ?.limit_gi || 100}{' '}
                          Gi
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              {/* Info Footer */}
              <div className="mt-4 pt-4 border-t border-border-dark/50 flex items-center gap-3">
                <span className="material-symbols-outlined text-blue-400 text-[16px] icon-fill">
                  info
                </span>
                <p className="text-[10px] text-text-secondary">
                  You can create services and manage configurations after the workspace is created.
                </p>
              </div>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
};

export default WorkspaceCreate;
