import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { api, Workspace } from '../services/api';

interface AdminStats {
  total_users: number;
  total_teams: number;
  total_workspaces: number;
  total_services: number;
  active_services: number;
  total_pvc_count: number;
  total_pvc_size_gi: number;
  total_cpu_cores: number;
  total_memory_gib: number;
  total_gpu_count: number;
}

const AdminWorkspaces: React.FC = () => {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [totalWorkspaces, setTotalWorkspaces] = useState(0);
  const limit = 10;

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page]);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);

      // Fetch stats
      const statsData = await api.get<AdminStats>('/api/admin/stats');
      setStats(statsData);

      // Fetch workspaces with pagination
      const offset = (page - 1) * limit;
      const data = await api.get<{ workspaces: Workspace[]; total: number }>(
        `/api/admin/workspaces?limit=${limit}&offset=${offset}`,
      );
      setWorkspaces(data.workspaces || []);
      setTotalWorkspaces(data.total);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load data';
      setError(message);
      console.error('Failed to load admin workspaces:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteWorkspace = async (id: string) => {
    if (
      !window.confirm(
        'Are you sure you want to delete this workspace? This action cannot be undone.',
      )
    ) {
      return;
    }

    try {
      await api.delete(`/api/workspaces/${id}`);
      await loadData();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to delete workspace: ' + message);
    }
  };

  const totalPages = Math.ceil(totalWorkspaces / limit);

  return (
    <div className="p-8 lg:p-12 max-w-[1400px] mx-auto space-y-8 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col gap-2">
        <h2 className="text-4xl font-bold tracking-tight">All Workspaces</h2>
        <p className="text-text-secondary max-w-2xl text-lg">
          View and manage all user workspaces across the platform.
        </p>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm">
          <div className="flex justify-between items-start mb-2">
            <p className="text-xs font-bold text-text-secondary uppercase tracking-widest">
              Total Teams & Users
            </p>
            <span className="material-symbols-outlined text-blue-500 text-[20px]">group</span>
          </div>
          <div className="flex items-baseline gap-2">
            <p className="text-3xl font-bold">{stats?.total_users ?? '-'}</p>
            <span className="text-sm text-text-secondary">users</span>
          </div>
          <p className="text-[10px] text-text-secondary mt-1">{stats?.total_teams ?? 0} Teams</p>
        </div>

        <div className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm">
          <div className="flex justify-between items-start mb-2">
            <p className="text-xs font-bold text-text-secondary uppercase tracking-widest">
              Total Workspaces
            </p>
            <span className="material-symbols-outlined text-primary text-[20px]">folder_open</span>
          </div>
          <p className="text-3xl font-bold">{stats?.total_workspaces ?? '-'}</p>
        </div>

        <div className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm">
          <div className="flex justify-between items-start mb-2">
            <p className="text-xs font-bold text-text-secondary uppercase tracking-widest">
              Total Services
            </p>
            <span className="material-symbols-outlined text-purple-500 text-[20px]">dns</span>
          </div>
          <p className="text-3xl font-bold">{stats?.total_services ?? '-'}</p>
        </div>

        <div className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm">
          <div className="flex justify-between items-start mb-2">
            <p className="text-xs font-bold text-text-secondary uppercase tracking-widest">
              Active Services
            </p>
            <span className="material-symbols-outlined text-emerald-500 text-[20px]">verified</span>
          </div>
          <div className="flex items-baseline gap-2">
            <p className="text-3xl font-bold">{stats?.active_services ?? '-'}</p>
            <span className="text-xs text-text-secondary">
              ({stats ? Math.round((stats.active_services / (stats.total_services || 1)) * 100) : 0}
              % online)
            </span>
          </div>
        </div>

        <div className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm">
          <div className="flex justify-between items-start mb-2">
            <p className="text-xs font-bold text-text-secondary uppercase tracking-widest">
              Storage (PVC)
            </p>
            <span className="material-symbols-outlined text-cyan-500 text-[20px]">storage</span>
          </div>
          <div className="flex items-baseline gap-2">
            <p className="text-3xl font-bold">{stats?.total_pvc_size_gi ?? '-'}</p>
            <span className="text-sm text-text-secondary">GiB</span>
          </div>
          <p className="text-[10px] text-text-secondary mt-1">{stats?.total_pvc_count ?? 0} PVCs</p>
        </div>

        <div className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm">
          <div className="flex justify-between items-start mb-2">
            <p className="text-xs font-bold text-text-secondary uppercase tracking-widest">
              CPU Cores
            </p>
            <span className="material-symbols-outlined text-orange-500 text-[20px]">memory</span>
          </div>
          <div className="flex items-baseline gap-2">
            <p className="text-3xl font-bold">{stats?.total_cpu_cores ?? '-'}</p>
            <span className="text-sm text-text-secondary">cores</span>
          </div>
        </div>

        <div className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm">
          <div className="flex justify-between items-start mb-2">
            <p className="text-xs font-bold text-text-secondary uppercase tracking-widest">
              Memory
            </p>
            <span className="material-symbols-outlined text-pink-500 text-[20px]">
              developer_board
            </span>
          </div>
          <div className="flex items-baseline gap-2">
            <p className="text-3xl font-bold">{stats?.total_memory_gib ?? '-'}</p>
            <span className="text-sm text-text-secondary">GiB</span>
          </div>
        </div>

        <div className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm">
          <div className="flex justify-between items-start mb-2">
            <p className="text-xs font-bold text-text-secondary uppercase tracking-widest">
              GPU Count
            </p>
            <span className="material-symbols-outlined text-yellow-500 text-[20px]">
              psychology
            </span>
          </div>
          <div className="flex items-baseline gap-2">
            <p className="text-3xl font-bold">{stats?.total_gpu_count ?? '-'}</p>
            <span className="text-sm text-text-secondary">GPUs</span>
          </div>
        </div>
      </div>

      {error && (
        <div className="rounded-xl bg-red-500/10 border border-red-500/20 p-6">
          <p className="text-red-500 font-semibold">{error}</p>
        </div>
      )}

      {/* Workspaces Table */}
      <div className="w-full overflow-hidden rounded-2xl border border-border-dark bg-surface-dark shadow-xl">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-background-dark/50 border-b border-border-dark">
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Workspace
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Owner
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Storage
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Services
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Created
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary text-right">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border-dark font-body text-sm">
              {loading && (
                <tr>
                  <td colSpan={6} className="p-8 text-center text-text-secondary">
                    <div className="flex items-center justify-center gap-2">
                      <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary"></div>
                      <span>Loading workspaces...</span>
                    </div>
                  </td>
                </tr>
              )}
              {!loading && workspaces.length === 0 && (
                <tr>
                  <td colSpan={6} className="p-8 text-center text-text-secondary">
                    <span className="material-symbols-outlined text-[32px] block mb-2 opacity-50">
                      folder_off
                    </span>
                    No workspaces found
                  </td>
                </tr>
              )}
              {!loading &&
                workspaces.map((ws) => (
                  <tr key={ws.id} className="hover:bg-white/5 transition-colors group">
                    <td className="p-5">
                      <div className="flex items-center gap-3">
                        <div className="size-10 rounded-lg bg-surface-dark flex items-center justify-center border border-white/5">
                          <span className="material-symbols-outlined text-[20px] text-primary">
                            folder
                          </span>
                        </div>
                        <div>
                          <Link
                            to={`/workspaces/${ws.id}`}
                            className="font-bold text-white text-sm hover:text-primary transition-colors"
                          >
                            {ws.name}
                          </Link>
                          <p className="text-[10px] text-text-secondary opacity-60 font-mono">
                            {ws.id}
                          </p>
                        </div>
                      </div>
                    </td>
                    <td className="p-5">
                      <div className="flex flex-col">
                        <span className="font-medium text-white">
                          {ws.owner?.username || 'Unknown'}
                        </span>
                        {ws.owner?.email && (
                          <span className="text-[10px] text-text-secondary">{ws.owner.email}</span>
                        )}
                      </div>
                    </td>
                    <td className="p-5">
                      <div className="flex flex-col">
                        <span className="font-bold">{ws.storage_size}</span>
                        <span className="text-[10px] text-text-secondary">{ws.storage_class}</span>
                      </div>
                    </td>
                    <td className="p-5">
                      <div className="flex flex-col gap-1">
                        <div className="flex items-center gap-2">
                          <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-widest bg-surface-highlight border border-white/5">
                            {ws.services?.length || 0} Total
                          </span>
                        </div>
                        <div className="flex items-center gap-1">
                          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-widest bg-emerald-500/10 text-emerald-500 border border-emerald-500/20">
                            <span className="material-symbols-outlined text-[12px]">
                              check_circle
                            </span>
                            {ws.services?.filter((s) => s.status === 'running').length || 0} Running
                          </span>
                        </div>
                      </div>
                    </td>
                    <td className="p-5 text-text-secondary">
                      {new Date(ws.created_at).toLocaleDateString()}
                    </td>
                    <td className="p-5 text-right">
                      <button
                        onClick={() => handleDeleteWorkspace(ws.id)}
                        className="text-text-secondary hover:text-red-500 transition-all p-1.5 rounded-lg hover:bg-white/10"
                        title="Delete workspace"
                      >
                        <span className="material-symbols-outlined text-[20px]">delete</span>
                      </button>
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        <div className="px-6 py-4 border-t border-border-dark flex items-center justify-between bg-background-dark/20">
          <p className="text-[11px] text-text-secondary font-medium">
            Showing {workspaces.length > 0 ? (page - 1) * limit + 1 : 0} to{' '}
            {Math.min(page * limit, totalWorkspaces)} of {totalWorkspaces} results
          </p>
          <div className="flex gap-2">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
              className="px-3 py-1.5 rounded-lg border border-border-dark text-xs font-bold text-text-secondary hover:bg-white/5 disabled:opacity-30 disabled:cursor-not-allowed transition-all"
            >
              Previous
            </button>
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
              className="px-3 py-1.5 rounded-lg border border-border-dark text-xs font-bold text-text-secondary hover:bg-white/5 disabled:opacity-30 disabled:cursor-not-allowed transition-all"
            >
              Next
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default AdminWorkspaces;
