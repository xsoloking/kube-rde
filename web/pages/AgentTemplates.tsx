import React, { useState, useEffect } from 'react';
import { agentTemplatesApi, AgentTemplate } from '../services/api';
import { useAuth } from '../contexts/AuthContext';

const JsonEditor: React.FC<{ label: string; value: unknown; onChange: (val: unknown) => void }> = ({
  label,
  value,
  onChange,
}) => {
  const [text, setText] = useState(value ? JSON.stringify(value, null, 2) : '');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // eslint-disable-next-line
    setText(value ? JSON.stringify(value, null, 2) : '');
  }, [value]);

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const newText = e.target.value;
    setText(newText);
    try {
      if (newText.trim() === '') {
        onChange(null);
        setError(null);
      } else {
        const parsed = JSON.parse(newText);
        onChange(parsed);
        setError(null);
      }
    } catch {
      setError('Invalid JSON');
    }
  };

  return (
    <div>
      <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
        {label} (JSON)
      </label>
      <textarea
        value={text}
        onChange={handleChange}
        className={`w-full px-4 py-3 bg-background-dark border rounded-lg text-white font-mono text-sm focus:outline-none transition-colors resize-none ${
          error ? 'border-red-500 focus:border-red-500' : 'border-border-dark focus:border-primary'
        }`}
        rows={5}
        placeholder="{}"
      />
      {error && <p className="text-xs text-red-500 mt-1">{error}</p>}
    </div>
  );
};

const AgentTemplates: React.FC = () => {
  const { user } = useAuth();
  const [templates, setTemplates] = useState<AgentTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editingTemplate, setEditingTemplate] = useState<AgentTemplate | null>(null);
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [isImportModalOpen, setIsImportModalOpen] = useState(false);
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState<string | null>(null);

  // Fetch templates on mount
  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);
      const templatesData = await agentTemplatesApi.list();
      setTemplates(templatesData);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load templates';
      setError(message);
      console.error('Failed to load templates:', err);
    } finally {
      setLoading(false);
    }
  };

  const isBuiltInTemplate = (templateId: string): boolean => {
    return templateId.startsWith('tpl-');
  };

  const isAdmin = (): boolean => {
    return user?.roles?.includes('admin') || false;
  };

  const handleEditTemplate = (template: AgentTemplate) => {
    setEditingTemplate({ ...template });
    setIsEditModalOpen(true);
  };

  const handleCreateTemplate = () => {
    setEditingTemplate({
      id: '',
      name: '',
      agent_type: 'web',
      description: '',
      docker_image: '',
      default_local_target: 'localhost:8080',
      default_external_port: 80,
      startup_args: '',
      env_vars: {},
      security_context: {},
      volume_mounts: [],
      created_at: '',
      updated_at: '',
    });
    setIsEditModalOpen(true);
  };

  const handleSaveTemplate = async () => {
    if (!editingTemplate) return;

    try {
      if (editingTemplate.id) {
        // Update existing
        await agentTemplatesApi.update(editingTemplate.id, {
          name: editingTemplate.name,
          description: editingTemplate.description,
          docker_image: editingTemplate.docker_image,
          default_local_target: editingTemplate.default_local_target,
          default_external_port: editingTemplate.default_external_port,
          startup_args: editingTemplate.startup_args,
          env_vars: editingTemplate.env_vars,
          security_context: editingTemplate.security_context,
          volume_mounts: editingTemplate.volume_mounts,
        });
      } else {
        // Create new
        await agentTemplatesApi.create({
          name: editingTemplate.name,
          agent_type: editingTemplate.agent_type,
          description: editingTemplate.description,
          docker_image: editingTemplate.docker_image,
          default_local_target: editingTemplate.default_local_target,
          default_external_port: editingTemplate.default_external_port,
          startup_args: editingTemplate.startup_args,
          env_vars: editingTemplate.env_vars,
          security_context: editingTemplate.security_context,
          volume_mounts: editingTemplate.volume_mounts,
        });
      }
      setIsEditModalOpen(false);
      setEditingTemplate(null);
      await loadData();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to save template: ' + message);
    }
  };

  const handleDeleteTemplate = async (templateId: string) => {
    if (
      !window.confirm(
        'Are you sure you want to delete this template? Services using this template will not be affected.',
      )
    ) {
      return;
    }

    try {
      await agentTemplatesApi.delete(templateId);
      await loadData();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to delete template: ' + message);
    }
  };

  const getAgentTypeIcon = (type: string) => {
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

  const handleExportAll = async () => {
    try {
      const response = await fetch('/api/agent-templates/export', {
        credentials: 'include',
      });

      if (!response.ok) {
        throw new Error('Failed to export templates');
      }

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'agent-templates-export.json';
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to export templates: ' + message);
    }
  };

  const handleExportSingle = async (templateId: string) => {
    try {
      const response = await fetch(`/api/agent-templates/${templateId}/export`, {
        credentials: 'include',
      });

      if (!response.ok) {
        throw new Error('Failed to export template');
      }

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `agent-template-${templateId}.json`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to export template: ' + message);
    }
  };

  const handleImport = async () => {
    if (!importFile) {
      alert('Please select a file to import');
      return;
    }

    try {
      setImporting(true);
      setImportResult(null);

      const fileContent = await importFile.text();
      const jsonData = JSON.parse(fileContent);

      const response = await fetch('/api/agent-templates/import', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify(jsonData),
      });

      if (!response.ok) {
        throw new Error('Failed to import templates');
      }

      const result = await response.json();
      setImportResult(result.message);

      // Reload templates
      await loadData();

      // Close modal after success
      setTimeout(() => {
        setIsImportModalOpen(false);
        setImportFile(null);
        setImportResult(null);
      }, 2000);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      setImportResult('Error: ' + message);
    } finally {
      setImporting(false);
    }
  };

  const builtInCount = templates.filter((t) => isBuiltInTemplate(t.id)).length;
  const customCount = templates.filter((t) => !isBuiltInTemplate(t.id)).length;

  return (
    <div className="p-8 lg:p-12 max-w-[1400px] mx-auto space-y-8 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-6">
        <div className="flex flex-col gap-2">
          <h2 className="text-4xl font-bold tracking-tight">Agent Templates</h2>
          <p className="text-text-secondary max-w-2xl text-lg">
            Manage agent templates for creating preconfigured services.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={handleExportAll}
            className="flex items-center gap-2 bg-surface-dark hover:bg-surface-highlight text-white border border-border-dark px-4 py-3 rounded-xl font-bold transition-all active:scale-95"
            title="Export all templates"
          >
            <span className="material-symbols-outlined text-[20px]">download</span>
            <span>Export</span>
          </button>
          <button
            onClick={() => setIsImportModalOpen(true)}
            className="flex items-center gap-2 bg-surface-dark hover:bg-surface-highlight text-white border border-border-dark px-4 py-3 rounded-xl font-bold transition-all active:scale-95"
            title="Import templates"
          >
            <span className="material-symbols-outlined text-[20px]">upload</span>
            <span>Import</span>
          </button>
          <button
            onClick={handleCreateTemplate}
            className="flex items-center gap-2 bg-primary hover:bg-primary-dark text-white px-6 py-3 rounded-xl font-bold shadow-xl shadow-primary/20 transition-all active:scale-95"
          >
            <span className="material-symbols-outlined text-[22px]">add</span>
            <span>Create Template</span>
          </button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm">
          <div className="flex justify-between items-start mb-2">
            <p className="text-xs font-bold text-text-secondary uppercase tracking-widest">
              Total Templates
            </p>
            <span className="material-symbols-outlined text-primary text-[20px]">dashboard</span>
          </div>
          <p className="text-3xl font-bold">{loading ? '-' : templates.length}</p>
        </div>

        <div className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm">
          <div className="flex justify-between items-start mb-2">
            <p className="text-xs font-bold text-text-secondary uppercase tracking-widest">
              Built-In
            </p>
            <span className="material-symbols-outlined text-emerald-500 text-[20px]">verified</span>
          </div>
          <p className="text-3xl font-bold">{loading ? '-' : builtInCount}</p>
        </div>

        <div className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm">
          <div className="flex justify-between items-start mb-2">
            <p className="text-xs font-bold text-text-secondary uppercase tracking-widest">
              Custom
            </p>
            <span className="material-symbols-outlined text-blue-500 text-[20px]">palette</span>
          </div>
          <p className="text-3xl font-bold">{loading ? '-' : customCount}</p>
        </div>
      </div>

      {error && (
        <div className="rounded-xl bg-red-500/10 border border-red-500/20 p-6">
          <p className="text-red-500 font-semibold">{error}</p>
        </div>
      )}

      {/* Templates Table */}
      <div className="w-full overflow-hidden rounded-2xl border border-border-dark bg-surface-dark shadow-xl">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-background-dark/50 border-b border-border-dark">
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Template
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Type
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Docker Image
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Default Port
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary text-right">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border-dark font-body text-sm">
              {loading && (
                <tr>
                  <td colSpan={5} className="p-8 text-center text-text-secondary">
                    <div className="flex items-center justify-center gap-2">
                      <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary"></div>
                      <span>Loading templates...</span>
                    </div>
                  </td>
                </tr>
              )}
              {!loading && templates.length === 0 && (
                <tr>
                  <td colSpan={5} className="p-8 text-center text-text-secondary">
                    <span className="material-symbols-outlined text-[32px] block mb-2 opacity-50">
                      deployed_code
                    </span>
                    No templates found
                  </td>
                </tr>
              )}
              {!loading &&
                templates.map((template) => {
                  const isBuiltIn = isBuiltInTemplate(template.id);

                  return (
                    <tr key={template.id} className="hover:bg-white/5 transition-colors group">
                      <td className="p-5">
                        <div className="flex items-center gap-3">
                          <div className="size-10 rounded-lg bg-surface-dark flex items-center justify-center border border-white/5">
                            <span className="material-symbols-outlined text-[20px] text-primary">
                              {getAgentTypeIcon(template.agent_type)}
                            </span>
                          </div>
                          <div>
                            <p className="font-bold text-white text-sm">{template.name}</p>
                            <p className="text-[10px] text-text-secondary opacity-60 font-mono">
                              {template.id}
                            </p>
                          </div>
                        </div>
                      </td>
                      <td className="p-5">
                        <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-widest bg-primary/20 text-primary">
                          {template.agent_type}
                        </span>
                      </td>
                      <td className="p-5">
                        <span className="font-mono text-[11px] text-text-secondary truncate max-w-xs">
                          {template.docker_image}
                        </span>
                      </td>
                      <td className="p-5">
                        <span className="font-bold text-[11px]">
                          {template.default_external_port}
                        </span>
                      </td>
                      <td className="p-5 text-right">
                        {isBuiltIn && !isAdmin() ? (
                          <span className="text-[10px] text-text-secondary/50 font-medium">
                            Built-in
                          </span>
                        ) : (
                          <div className="flex items-center justify-end gap-2">
                            <button
                              onClick={() => handleExportSingle(template.id)}
                              className="text-text-secondary hover:text-blue-500 transition-all p-1.5 rounded-lg hover:bg-white/10"
                              title="Export template"
                            >
                              <span className="material-symbols-outlined text-[20px]">
                                download
                              </span>
                            </button>
                            <button
                              onClick={() => handleEditTemplate(template)}
                              className="text-text-secondary hover:text-primary transition-all p-1.5 rounded-lg hover:bg-white/10"
                              title="Edit template"
                            >
                              <span className="material-symbols-outlined text-[20px]">edit</span>
                            </button>
                            {!isBuiltIn && (
                              <button
                                onClick={() => handleDeleteTemplate(template.id)}
                                className="text-text-secondary hover:text-red-500 transition-all p-1.5 rounded-lg hover:bg-white/10"
                                title="Delete template"
                              >
                                <span className="material-symbols-outlined text-[20px]">
                                  delete
                                </span>
                              </button>
                            )}
                            {isBuiltIn && isAdmin() && (
                              <span className="text-[9px] text-blue-400 bg-blue-500/10 px-2 py-1 rounded border border-blue-500/20 font-bold uppercase tracking-widest">
                                Built-in
                              </span>
                            )}
                          </div>
                        )}
                      </td>
                    </tr>
                  );
                })}
            </tbody>
          </table>
        </div>

        {/* Table Footer */}
        {!loading && (
          <div className="px-6 py-4 border-t border-border-dark flex items-center justify-between bg-background-dark/20">
            <p className="text-[11px] text-text-secondary font-medium">
              Showing {templates.length} template{templates.length !== 1 ? 's' : ''}
            </p>
          </div>
        )}
      </div>

      {/* Info Banner */}
      <div className="bg-blue-500/10 rounded-xl p-6 border border-blue-500/20 flex items-start gap-4">
        <span className="material-symbols-outlined text-blue-400 text-2xl mt-1 flex-shrink-0">
          info
        </span>
        <div>
          <p className="text-blue-400 font-semibold mb-1">Built-in templates are protected</p>
          <p className="text-blue-400/70 text-sm">
            Built-in templates (SSH, File Server, Coder, Jupyter) cannot be deleted. Custom
            templates can be removed if not in use.
          </p>
        </div>
      </div>

      {/* Edit Template Modal */}
      {isEditModalOpen && editingTemplate && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
          <div className="bg-surface-dark rounded-2xl border border-border-dark shadow-2xl max-w-2xl w-full max-h-[90vh] overflow-y-auto">
            {/* Modal Header */}
            <div className="sticky top-0 bg-surface-dark border-b border-border-dark p-6 flex items-center justify-between">
              <div>
                <h3 className="text-2xl font-bold">
                  {editingTemplate.id ? 'Edit Template' : 'Create Template'}
                </h3>
                <p className="text-text-secondary text-sm mt-1">
                  {editingTemplate.id
                    ? 'Modify template configuration'
                    : 'Define a new agent template'}
                </p>
              </div>
              <button
                onClick={() => setIsEditModalOpen(false)}
                className="text-text-secondary hover:text-white transition-colors p-2 rounded-lg hover:bg-white/10"
              >
                <span className="material-symbols-outlined text-[24px]">close</span>
              </button>
            </div>

            {/* Modal Body */}
            <div className="p-6 space-y-6">
              {/* Template Name */}
              <div>
                <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
                  Template Name
                </label>
                <input
                  type="text"
                  value={editingTemplate.name}
                  onChange={(e) => setEditingTemplate({ ...editingTemplate, name: e.target.value })}
                  className="w-full px-4 py-3 bg-background-dark border border-border-dark rounded-lg text-white focus:outline-none focus:border-primary transition-colors"
                  placeholder="Template name"
                />
              </div>

              {/* Agent Type */}
              <div>
                <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
                  Agent Type
                </label>
                <div className="relative">
                  <select
                    value={editingTemplate.agent_type}
                    onChange={(e) =>
                      setEditingTemplate({ ...editingTemplate, agent_type: e.target.value })
                    }
                    className="w-full px-4 py-3 bg-background-dark border border-border-dark rounded-lg text-white focus:outline-none focus:border-primary transition-colors appearance-none cursor-pointer"
                    disabled={!!editingTemplate.id && isBuiltInTemplate(editingTemplate.id)}
                  >
                    <option value="web">Web Service (Generic)</option>
                    <option value="ssh">SSH Server</option>
                    <option value="file">File Browser</option>
                    <option value="coder">Code Server</option>
                    <option value="jupyter">Jupyter Notebook</option>
                  </select>
                  <div className="absolute right-4 top-1/2 -translate-y-1/2 pointer-events-none text-text-secondary">
                    <span className="material-symbols-outlined">expand_more</span>
                  </div>
                </div>
              </div>

              {/* Description */}
              <div>
                <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
                  Description
                </label>
                <textarea
                  value={editingTemplate.description || ''}
                  onChange={(e) =>
                    setEditingTemplate({ ...editingTemplate, description: e.target.value })
                  }
                  className="w-full px-4 py-3 bg-background-dark border border-border-dark rounded-lg text-white focus:outline-none focus:border-primary transition-colors resize-none"
                  rows={3}
                  placeholder="Template description"
                />
              </div>

              {/* Docker Image */}
              <div>
                <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
                  Docker Image
                </label>
                <input
                  type="text"
                  value={editingTemplate.docker_image}
                  onChange={(e) =>
                    setEditingTemplate({ ...editingTemplate, docker_image: e.target.value })
                  }
                  className="w-full px-4 py-3 bg-background-dark border border-border-dark rounded-lg text-white font-mono text-sm focus:outline-none focus:border-primary transition-colors"
                  placeholder="e.g., ubuntu:22.04"
                />
              </div>

              {/* Default Local Target */}
              <div>
                <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
                  Default Local Target
                </label>
                <input
                  type="text"
                  value={editingTemplate.default_local_target}
                  onChange={(e) =>
                    setEditingTemplate({ ...editingTemplate, default_local_target: e.target.value })
                  }
                  className="w-full px-4 py-3 bg-background-dark border border-border-dark rounded-lg text-white font-mono text-sm focus:outline-none focus:border-primary transition-colors"
                  placeholder="e.g., localhost:22"
                />
              </div>

              {/* Default External Port */}
              <div>
                <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
                  Default External Port
                </label>
                <input
                  type="number"
                  value={editingTemplate.default_external_port}
                  onChange={(e) =>
                    setEditingTemplate({
                      ...editingTemplate,
                      default_external_port: parseInt(e.target.value) || 0,
                    })
                  }
                  className="w-full px-4 py-3 bg-background-dark border border-border-dark rounded-lg text-white focus:outline-none focus:border-primary transition-colors"
                  placeholder="e.g., 22"
                  min="1"
                  max="65535"
                />
              </div>

              {/* Startup Args */}
              <div>
                <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
                  Startup Args
                </label>
                <input
                  type="text"
                  value={editingTemplate.startup_args || ''}
                  onChange={(e) =>
                    setEditingTemplate({ ...editingTemplate, startup_args: e.target.value })
                  }
                  className="w-full px-4 py-3 bg-background-dark border border-border-dark rounded-lg text-white font-mono text-sm focus:outline-none focus:border-primary transition-colors"
                  placeholder="e.g., /usr/sbin/sshd -D"
                />
              </div>

              {/* Environment Variables (JSON) */}
              <div>
                <JsonEditor
                  label="Environment Variables"
                  value={editingTemplate.env_vars}
                  onChange={(newVal) =>
                    setEditingTemplate({
                      ...editingTemplate,
                      env_vars: newVal as Record<string, unknown>,
                    })
                  }
                />
              </div>

              {/* Security Context (JSON) */}
              <div>
                <JsonEditor
                  label="Security Context"
                  value={editingTemplate.security_context}
                  onChange={(newVal) =>
                    setEditingTemplate({
                      ...editingTemplate,
                      security_context: newVal as Record<string, unknown>,
                    })
                  }
                />
              </div>

              {/* Volume Mounts (JSON) */}
              <div>
                <JsonEditor
                  label="Volume Mounts"
                  value={editingTemplate.volume_mounts}
                  onChange={(newVal) =>
                    setEditingTemplate({ ...editingTemplate, volume_mounts: newVal as unknown[] })
                  }
                />
              </div>
            </div>

            {/* Modal Footer */}
            <div className="sticky bottom-0 bg-surface-dark border-t border-border-dark p-6 flex items-center justify-end gap-3">
              <button
                onClick={() => setIsEditModalOpen(false)}
                className="px-6 py-2.5 bg-surface-highlight hover:bg-white/10 text-white text-sm font-bold uppercase tracking-widest rounded-lg border border-border-dark transition-all"
              >
                Cancel
              </button>
              <button
                onClick={handleSaveTemplate}
                className="px-6 py-2.5 bg-primary hover:bg-primary-dark text-white text-sm font-bold uppercase tracking-widest rounded-lg shadow-lg shadow-primary/20 transition-all"
              >
                Save Changes
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Import Modal */}
      {isImportModalOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-surface-dark border border-border-dark rounded-2xl shadow-2xl w-full max-w-lg p-8">
            <div className="flex items-center justify-between mb-6">
              <h3 className="text-2xl font-bold">Import Templates</h3>
              <button
                onClick={() => {
                  setIsImportModalOpen(false);
                  setImportFile(null);
                  setImportResult(null);
                }}
                className="text-text-secondary hover:text-white p-2 rounded-lg hover:bg-white/10 transition-all"
              >
                <span className="material-symbols-outlined text-[24px]">close</span>
              </button>
            </div>

            <div className="space-y-6">
              <div>
                <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
                  Select JSON File
                </label>
                <div className="flex items-center gap-3">
                  <input
                    type="file"
                    accept=".json,application/json"
                    onChange={(e) => setImportFile(e.target.files?.[0] || null)}
                    className="hidden"
                    id="import-file-input"
                  />
                  <label
                    htmlFor="import-file-input"
                    className="flex-1 px-4 py-3 bg-background-dark border border-border-dark rounded-lg text-text-secondary cursor-pointer hover:border-primary transition-colors flex items-center gap-2"
                  >
                    <span className="material-symbols-outlined text-[20px]">upload_file</span>
                    <span className="text-sm truncate">{importFile?.name || 'Choose file...'}</span>
                  </label>
                </div>
                <p className="text-[10px] text-text-secondary mt-2">
                  Upload a JSON file exported from this system
                </p>
              </div>

              {importResult && (
                <div
                  className={`p-4 rounded-lg border ${importResult.startsWith('Error') ? 'bg-red-500/10 border-red-500/20 text-red-500' : 'bg-emerald-500/10 border-emerald-500/20 text-emerald-500'}`}
                >
                  <p className="text-sm font-medium">{importResult}</p>
                </div>
              )}

              <div className="flex items-center gap-3 pt-4">
                <button
                  onClick={() => {
                    setIsImportModalOpen(false);
                    setImportFile(null);
                    setImportResult(null);
                  }}
                  className="flex-1 px-4 py-2.5 bg-surface-highlight hover:bg-white/10 text-white text-sm font-bold uppercase tracking-widest rounded-lg border border-border-dark transition-all"
                >
                  Cancel
                </button>
                <button
                  onClick={handleImport}
                  disabled={!importFile || importing}
                  className="flex-1 px-4 py-2.5 bg-primary hover:bg-primary-dark text-white text-sm font-bold uppercase tracking-widest rounded-lg shadow-lg shadow-primary/20 transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                >
                  {importing ? (
                    <>
                      <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
                      <span>Importing...</span>
                    </>
                  ) : (
                    <>
                      <span className="material-symbols-outlined text-[18px]">upload</span>
                      <span>Import</span>
                    </>
                  )}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default AgentTemplates;
