import React, { useState, useEffect } from 'react';
import {
  resourceConfigApi,
  ResourceConfig,
  StorageClassConfig,
  GPUTypeConfig,
} from '../services/api';

const ResourceManagement: React.FC = () => {
  const [config, setConfig] = useState<ResourceConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      setLoading(true);
      const data = await resourceConfigApi.get();

      // Parse JSONB strings if needed
      if (typeof data.storage_classes === 'string') {
        data.storage_classes = JSON.parse(data.storage_classes);
      }
      if (typeof data.gpu_types === 'string') {
        data.gpu_types = JSON.parse(data.gpu_types);
      }

      setConfig(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      setError('Failed to load resource config: ' + message);
      console.error('Failed to load config:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    if (!config) return;

    setSaving(true);
    setError(null);
    setSuccess(false);

    try {
      // Convert arrays to JSON strings for backend
      const configToSave = {
        ...config,
        storage_classes: JSON.stringify(config.storage_classes),
        gpu_types: JSON.stringify(config.gpu_types),
      };

      await resourceConfigApi.update(configToSave);
      setSuccess(true);
      setTimeout(() => setSuccess(false), 3000);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to save config: ' + message);
      console.error('Failed to save config:', err);
    } finally {
      setSaving(false);
    }
  };

  const addStorageClass = () => {
    if (!config) return;
    const storageClasses = Array.isArray(config.storage_classes) ? [...config.storage_classes] : [];
    storageClasses.push({ name: '', limit_gi: 20, is_default: false });
    setConfig({ ...config, storage_classes: storageClasses });
  };

  const removeStorageClass = (index: number) => {
    if (!config || !Array.isArray(config.storage_classes)) return;
    const storageClasses = [...config.storage_classes];
    storageClasses.splice(index, 1);
    setConfig({ ...config, storage_classes: storageClasses });
  };

  const updateStorageClass = (
    index: number,
    field: keyof StorageClassConfig,
    value: string | number | boolean,
  ) => {
    if (!config || !Array.isArray(config.storage_classes)) return;
    const storageClasses = [...config.storage_classes];
    storageClasses[index] = { ...storageClasses[index], [field]: value };
    setConfig({ ...config, storage_classes: storageClasses });
  };

  const setStorageClassDefault = (index: number) => {
    if (!config || !Array.isArray(config.storage_classes)) return;
    const storageClasses = config.storage_classes.map((sc, i) => ({
      ...sc,
      is_default: i === index,
    }));
    setConfig({ ...config, storage_classes: storageClasses });
  };

  const addGPUType = () => {
    if (!config) return;
    const gpuTypes = Array.isArray(config.gpu_types) ? [...config.gpu_types] : [];
    gpuTypes.push({
      model_name: '',
      resource_name: '',
      node_label_key: '',
      node_label_value: '',
      limit: 0,
      is_default: false,
    });
    setConfig({ ...config, gpu_types: gpuTypes });
  };

  const removeGPUType = (index: number) => {
    if (!config || !Array.isArray(config.gpu_types)) return;
    const gpuTypes = [...config.gpu_types];
    gpuTypes.splice(index, 1);
    setConfig({ ...config, gpu_types: gpuTypes });
  };

  const updateGPUType = (
    index: number,
    field: keyof GPUTypeConfig,
    value: string | number | boolean,
  ) => {
    if (!config || !Array.isArray(config.gpu_types)) return;
    const gpuTypes = [...config.gpu_types];
    gpuTypes[index] = { ...gpuTypes[index], [field]: value };
    setConfig({ ...config, gpu_types: gpuTypes });
  };

  const setGPUTypeDefault = (index: number) => {
    if (!config || !Array.isArray(config.gpu_types)) return;
    const gpuTypes = config.gpu_types.map((gpu, i) => ({
      ...gpu,
      is_default: i === index,
    }));
    setConfig({ ...config, gpu_types: gpuTypes });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-white">Loading...</div>
      </div>
    );
  }

  if (!config) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-red-400">Failed to load configuration</div>
      </div>
    );
  }

  const storageClasses = Array.isArray(config.storage_classes) ? config.storage_classes : [];
  const gpuTypes = Array.isArray(config.gpu_types) ? config.gpu_types : [];

  return (
    <div className="min-h-screen bg-background-dark p-8">
      <div className="max-w-4xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-white mb-2">Resource Management</h1>
          <p className="text-text-secondary">
            Configure system-wide resource quotas and defaults for new users
          </p>
        </div>

        {/* Success Message */}
        {success && (
          <div className="mb-6 p-4 bg-green-500/10 border border-green-500/20 rounded-lg text-green-400">
            Configuration saved successfully
          </div>
        )}

        {/* Error Message */}
        {error && (
          <div className="mb-6 p-4 bg-red-500/10 border border-red-500/20 rounded-lg text-red-400">
            {error}
          </div>
        )}

        <div className="space-y-6">
          {/* Default User Quotas Section - All Resources */}
          <div className="bg-surface-dark border border-border-dark rounded-2xl p-6">
            <div className="flex items-center gap-3 mb-6 pb-4 border-b border-border-dark">
              <span className="material-symbols-outlined text-primary text-2xl">settings</span>
              <div>
                <h2 className="text-xl font-bold text-white">Default User Quotas</h2>
                <p className="text-text-secondary text-sm mt-1">
                  Configure default resource limits for new users
                </p>
              </div>
            </div>

            <div className="space-y-8">
              {/* CPU & Memory */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-text-secondary mb-2 flex items-center gap-2">
                    <span className="material-symbols-outlined text-lg">memory</span>
                    CPU Cores
                  </label>
                  <input
                    type="number"
                    min="0"
                    value={config.default_cpu_cores}
                    onChange={(e) =>
                      setConfig({ ...config, default_cpu_cores: parseInt(e.target.value) || 0 })
                    }
                    className="w-full px-4 py-2 bg-background-dark border border-border-dark rounded-lg text-white focus:border-primary focus:outline-none"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-text-secondary mb-2 flex items-center gap-2">
                    <span className="material-symbols-outlined text-lg">memory</span>
                    Memory (Gi)
                  </label>
                  <input
                    type="number"
                    min="0"
                    value={config.default_memory_gi}
                    onChange={(e) =>
                      setConfig({ ...config, default_memory_gi: parseInt(e.target.value) || 0 })
                    }
                    className="w-full px-4 py-2 bg-background-dark border border-border-dark rounded-lg text-white focus:border-primary focus:outline-none"
                  />
                </div>
              </div>

              {/* Storage Configuration */}
              <div>
                <div className="flex items-center justify-between mb-3">
                  <label className="text-sm font-medium text-text-secondary flex items-center gap-2">
                    <span className="material-symbols-outlined text-lg">storage</span>
                    Storage Configuration
                  </label>
                  <button
                    onClick={addStorageClass}
                    className="flex items-center gap-1 px-3 py-1 text-xs bg-primary/10 hover:bg-primary/20 text-primary rounded-lg transition-colors"
                  >
                    <span className="material-symbols-outlined text-sm">add</span>
                    Add Storage Class
                  </button>
                </div>
                <div className="bg-background-dark border border-border-dark rounded-lg p-4">
                  {storageClasses.length > 0 ? (
                    <div className="space-y-2">
                      <label className="block text-xs text-text-secondary mb-2">
                        Storage Classes
                      </label>
                      {storageClasses.map((sc, index) => (
                        <div key={index} className="flex items-center gap-2">
                          <input
                            type="radio"
                            name="default_storage"
                            checked={sc.is_default || false}
                            onChange={() => setStorageClassDefault(index)}
                            className="w-4 h-4 text-primary focus:ring-primary"
                            title="Set as default"
                          />
                          <input
                            type="text"
                            placeholder="Class name (e.g., local-path)"
                            value={sc.name}
                            onChange={(e) => updateStorageClass(index, 'name', e.target.value)}
                            className="flex-1 px-3 py-1.5 bg-surface-dark border border-border-dark rounded-lg text-white text-sm focus:border-primary focus:outline-none"
                          />
                          <input
                            type="number"
                            min="0"
                            placeholder="Limit (Gi)"
                            value={sc.limit_gi}
                            onChange={(e) =>
                              updateStorageClass(index, 'limit_gi', parseInt(e.target.value) || 0)
                            }
                            className="w-24 px-3 py-1.5 bg-surface-dark border border-border-dark rounded-lg text-white text-sm focus:border-primary focus:outline-none"
                          />
                          <button
                            onClick={() => removeStorageClass(index)}
                            className="p-1.5 text-red-400 hover:bg-red-400/10 rounded-lg transition-colors"
                          >
                            <span className="material-symbols-outlined text-sm">close</span>
                          </button>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="text-sm text-text-secondary text-center py-4">
                      No storage classes configured. Click "Add Storage Class" to get started.
                    </p>
                  )}
                </div>
              </div>

              {/* GPU Configuration */}
              <div>
                <div className="flex items-center justify-between mb-3">
                  <label className="text-sm font-medium text-text-secondary flex items-center gap-2">
                    <span className="material-symbols-outlined text-lg">videogame_asset</span>
                    GPU Configuration
                  </label>
                  <button
                    onClick={addGPUType}
                    className="flex items-center gap-1 px-3 py-1 text-xs bg-primary/10 hover:bg-primary/20 text-primary rounded-lg transition-colors"
                  >
                    <span className="material-symbols-outlined text-sm">add</span>
                    Add GPU Type
                  </button>
                </div>
                <div className="bg-background-dark border border-border-dark rounded-lg p-4">
                  {gpuTypes.length > 0 ? (
                    <div className="space-y-4">
                      <label className="block text-xs text-text-secondary mb-2">GPU Types</label>
                      {gpuTypes.map((gpu, index) => (
                        <div
                          key={index}
                          className="bg-surface-dark border border-border-dark rounded-lg p-4 space-y-3"
                        >
                          <div className="flex items-start justify-between gap-2">
                            <div className="flex items-center gap-2">
                              <input
                                type="radio"
                                name="default_gpu"
                                checked={gpu.is_default || false}
                                onChange={() => setGPUTypeDefault(index)}
                                className="w-4 h-4 text-primary focus:ring-primary mt-1"
                                title="Set as default"
                              />
                              <div className="flex-1">
                                <label className="block text-[10px] text-text-secondary mb-1 font-bold uppercase tracking-widest">
                                  Model Name
                                </label>
                                <input
                                  type="text"
                                  placeholder="e.g., NVIDIA A100"
                                  value={gpu.model_name}
                                  onChange={(e) =>
                                    updateGPUType(index, 'model_name', e.target.value)
                                  }
                                  className="w-full px-3 py-1.5 bg-background-dark border border-border-dark rounded-lg text-white text-sm focus:border-primary focus:outline-none"
                                />
                              </div>
                            </div>
                            <button
                              onClick={() => removeGPUType(index)}
                              className="p-1.5 text-red-400 hover:bg-red-400/10 rounded-lg transition-colors"
                            >
                              <span className="material-symbols-outlined text-sm">close</span>
                            </button>
                          </div>

                          <div className="grid grid-cols-2 gap-3">
                            <div>
                              <label className="block text-[10px] text-text-secondary mb-1 font-bold uppercase tracking-widest">
                                Resource Name
                              </label>
                              <input
                                type="text"
                                placeholder="e.g., nvidia.com/gpu"
                                value={gpu.resource_name}
                                onChange={(e) =>
                                  updateGPUType(index, 'resource_name', e.target.value)
                                }
                                className="w-full px-3 py-1.5 bg-background-dark border border-border-dark rounded-lg text-white text-sm focus:border-primary focus:outline-none font-mono"
                              />
                            </div>
                            <div>
                              <label className="block text-[10px] text-text-secondary mb-1 font-bold uppercase tracking-widest">
                                Limit
                              </label>
                              <input
                                type="number"
                                min="0"
                                placeholder="Max GPUs"
                                value={gpu.limit}
                                onChange={(e) =>
                                  updateGPUType(index, 'limit', parseInt(e.target.value) || 0)
                                }
                                className="w-full px-3 py-1.5 bg-background-dark border border-border-dark rounded-lg text-white text-sm focus:border-primary focus:outline-none"
                              />
                            </div>
                          </div>

                          <div className="grid grid-cols-2 gap-3">
                            <div>
                              <label className="block text-[10px] text-text-secondary mb-1 font-bold uppercase tracking-widest">
                                Node Label Key
                              </label>
                              <input
                                type="text"
                                placeholder="e.g., nvidia.com/model"
                                value={gpu.node_label_key}
                                onChange={(e) =>
                                  updateGPUType(index, 'node_label_key', e.target.value)
                                }
                                className="w-full px-3 py-1.5 bg-background-dark border border-border-dark rounded-lg text-white text-sm focus:border-primary focus:outline-none font-mono"
                              />
                            </div>
                            <div>
                              <label className="block text-[10px] text-text-secondary mb-1 font-bold uppercase tracking-widest">
                                Node Label Value
                              </label>
                              <input
                                type="text"
                                placeholder="e.g., A100"
                                value={gpu.node_label_value}
                                onChange={(e) =>
                                  updateGPUType(index, 'node_label_value', e.target.value)
                                }
                                className="w-full px-3 py-1.5 bg-background-dark border border-border-dark rounded-lg text-white text-sm focus:border-primary focus:outline-none font-mono"
                              />
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="text-sm text-text-secondary text-center py-4">
                      No GPU types configured. Click "Add GPU Type" to get started.
                    </p>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* Save Button */}
          <div className="flex justify-end">
            <button
              onClick={handleSave}
              disabled={saving}
              className="flex items-center gap-2 px-6 py-3 bg-primary hover:bg-primary/80 disabled:bg-gray-600 text-white rounded-lg transition-colors font-medium"
            >
              {saving ? (
                <>
                  <span className="material-symbols-outlined animate-spin">refresh</span>
                  Saving...
                </>
              ) : (
                <>
                  <span className="material-symbols-outlined">save</span>
                  Save Configuration
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ResourceManagement;
