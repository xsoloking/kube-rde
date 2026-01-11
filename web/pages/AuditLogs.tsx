import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { auditApi, AuditLog } from '../services/api';

const AuditLogs: React.FC = () => {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const pageSize = 20;

  // Filters
  const [userId, setUserId] = useState('');
  const [action, setAction] = useState('');
  const [resource, setResource] = useState('');
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');

  const fetchLogs = async () => {
    try {
      setLoading(true);
      setError(null);
      const offset = (page - 1) * pageSize;
      const response = await auditApi.list({
        limit: pageSize,
        offset,
        user_id: userId || undefined,
        action: action || undefined,
        resource: resource || undefined,
        start_date: startDate ? new Date(startDate).toISOString() : undefined,
        end_date: endDate ? new Date(endDate).toISOString() : undefined,
      });
      setLogs(response.logs || []);
      setTotal(response.total || 0);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load audit logs';
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchLogs();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page]); // Reload when page changes

  const handleFilterSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1); // Reset to first page
    fetchLogs();
  };

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className="p-8 lg:p-12 max-w-[1400px] mx-auto space-y-8 animate-fade-in">
      <div className="flex flex-col gap-2">
        <h2 className="text-4xl font-bold tracking-tight text-white">Audit Logs</h2>
        <p className="text-text-secondary max-w-2xl text-lg">
          Track user actions and system events for compliance and security.
        </p>
      </div>

      {/* Filters */}
      <form
        onSubmit={handleFilterSubmit}
        className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm space-y-4"
      >
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
          <div>
            <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
              User ID
            </label>
            <input
              type="text"
              value={userId}
              onChange={(e) => setUserId(e.target.value)}
              placeholder="Filter by User ID..."
              className="w-full h-10 px-3 bg-background-dark border border-border-dark rounded-lg text-white focus:border-primary focus:outline-none text-sm"
            />
          </div>
          <div>
            <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
              Action
            </label>
            <select
              value={action}
              onChange={(e) => setAction(e.target.value)}
              className="w-full h-10 px-3 bg-background-dark border border-border-dark rounded-lg text-white focus:border-primary focus:outline-none text-sm appearance-none cursor-pointer"
            >
              <option value="">All Actions</option>
              <option value="create">Create</option>
              <option value="update">Update</option>
              <option value="delete">Delete</option>
              <option value="login">Login</option>
            </select>
          </div>
          <div>
            <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
              Resource
            </label>
            <select
              value={resource}
              onChange={(e) => setResource(e.target.value)}
              className="w-full h-10 px-3 bg-background-dark border border-border-dark rounded-lg text-white focus:border-primary focus:outline-none text-sm appearance-none cursor-pointer"
            >
              <option value="">All Resources</option>
              <option value="user">User</option>
              <option value="workspace">Workspace</option>
              <option value="service">Service</option>
              <option value="agent">Agent</option>
            </select>
          </div>
          <div>
            <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
              Start Date
            </label>
            <input
              type="datetime-local"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              className="w-full h-10 px-3 bg-background-dark border border-border-dark rounded-lg text-white focus:border-primary focus:outline-none text-sm"
            />
          </div>
          <div>
            <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
              End Date
            </label>
            <input
              type="datetime-local"
              value={endDate}
              onChange={(e) => setEndDate(e.target.value)}
              className="w-full h-10 px-3 bg-background-dark border border-border-dark rounded-lg text-white focus:border-primary focus:outline-none text-sm"
            />
          </div>
        </div>
        <div className="flex justify-end pt-2">
          <button
            type="submit"
            className="flex items-center gap-2 px-6 py-2 bg-primary hover:bg-primary-dark text-white rounded-lg font-bold text-sm transition-all"
          >
            <span className="material-symbols-outlined text-[18px]">search</span>
            Search Logs
          </button>
        </div>
      </form>

      {error && (
        <div className="rounded-xl bg-red-500/10 border border-red-500/20 p-6 text-red-500 font-semibold">
          {error}
        </div>
      )}

      {/* Table */}
      <div className="w-full overflow-hidden rounded-2xl border border-border-dark bg-surface-dark shadow-xl">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-background-dark/50 border-b border-border-dark">
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Time
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  User
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Action
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Resource
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Resource ID
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Details
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border-dark font-body text-sm">
              {loading ? (
                <tr>
                  <td colSpan={6} className="p-8 text-center text-text-secondary">
                    <div className="flex items-center justify-center gap-2">
                      <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary"></div>
                      <span>Loading logs...</span>
                    </div>
                  </td>
                </tr>
              ) : logs.length === 0 ? (
                <tr>
                  <td colSpan={6} className="p-8 text-center text-text-secondary">
                    <span className="material-symbols-outlined text-[32px] block mb-2 opacity-50">
                      history_edu
                    </span>
                    No audit logs found matching your criteria
                  </td>
                </tr>
              ) : (
                logs.map((log) => (
                  <tr key={log.id} className="hover:bg-white/5 transition-colors group">
                    <td className="p-5 text-text-secondary whitespace-nowrap">
                      {new Date(log.timestamp).toLocaleString()}
                    </td>
                    <td className="p-5 font-medium text-white">
                      {log.user ? (
                        <Link
                          to={`/users/${log.user.id}`}
                          className="hover:text-primary transition-colors"
                        >
                          {log.user.username}
                        </Link>
                      ) : (
                        <span className="text-text-secondary">{log.user_id}</span>
                      )}
                    </td>
                    <td className="p-5">
                      <span
                        className={`inline-flex items-center px-2 py-1 rounded-full text-[10px] font-bold uppercase tracking-widest ${
                          log.action === 'create'
                            ? 'bg-emerald-500/10 text-emerald-500 border border-emerald-500/20'
                            : log.action === 'delete'
                              ? 'bg-red-500/10 text-red-500 border border-red-500/20'
                              : log.action === 'update'
                                ? 'bg-blue-500/10 text-blue-500 border border-blue-500/20'
                                : 'bg-surface-highlight text-text-secondary border border-white/10'
                        }`}
                      >
                        {log.action}
                      </span>
                    </td>
                    <td className="p-5 capitalize">{log.resource}</td>
                    <td className="p-5 font-mono text-xs text-text-secondary">{log.resource_id}</td>
                    <td className="p-5">
                      <button
                        onClick={() =>
                          alert(JSON.stringify({ old: log.old_data, new: log.new_data }, null, 2))
                        }
                        className="text-text-secondary hover:text-primary transition-colors"
                        title="View JSON Diff"
                      >
                        <span className="material-symbols-outlined">data_object</span>
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {!loading && (
          <div className="px-6 py-4 border-t border-border-dark flex items-center justify-between bg-background-dark/20">
            <p className="text-[11px] text-text-secondary font-medium">
              Showing {logs.length > 0 ? (page - 1) * pageSize + 1 : 0} to{' '}
              {Math.min(page * pageSize, total)} of {total} results
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
        )}
      </div>
    </div>
  );
};

export default AuditLogs;
