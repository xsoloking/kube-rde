import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { workspacesApi, Workspace, Service } from '../services/api';

const Workspaces: React.FC = () => {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [sortBy, setSortBy] = useState<'newest' | 'name'>('newest');

  useEffect(() => {
    const fetchWorkspaces = async () => {
      try {
        setLoading(true);
        setError(null);
        const data = await workspacesApi.list();
        setWorkspaces(data);
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to load workspaces';
        setError(message);
        console.error('Failed to load workspaces:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchWorkspaces();
  }, []);

  const handleDeleteWorkspace = async (id: string) => {
    if (!window.confirm('Are you sure you want to delete this workspace?')) return;

    try {
      await workspacesApi.delete(id);
      setWorkspaces(workspaces.filter((ws) => ws.id !== id));
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to delete workspace';
      setError(message);
    }
  };

  // Filter workspaces by search query
  const filteredWorkspaces = workspaces.filter(
    (ws) =>
      ws.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      ws.id.toLowerCase().includes(searchQuery.toLowerCase()),
  );

  // Sort workspaces
  const sortedWorkspaces = [...filteredWorkspaces].sort((a, b) => {
    if (sortBy === 'newest') {
      return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
    } else {
      return a.name.localeCompare(b.name);
    }
  });

  const getServiceIcon = (type?: string) => {
    switch (type) {
      case 'ssh':
        return 'terminal';
      case 'file':
        return 'folder_open';
      case 'coder':
        return 'code';
      case 'jupyter':
        return 'science';
      default:
        return 'deployed_code';
    }
  };

  const getServiceColor = (type?: string) => {
    switch (type) {
      case 'ssh':
        return 'text-emerald-500';
      case 'file':
        return 'text-yellow-500';
      case 'coder':
        return 'text-blue-500';
      case 'jupyter':
        return 'text-orange-500';
      default:
        return 'text-primary';
    }
  };

  const handleServiceClick = (e: React.MouseEvent, service: Service) => {
    e.preventDefault();
    if (service.agent_type === 'ssh') {
      // Navigate to service detail
      window.location.href = `/#/services/${service.id}`;
    } else {
      // Open Web Access URL using remote_proxy from server
      const url = service.remote_proxy || `${service.agent_id}.192-168-97-2.nip.io`;
      window.open(`http://${url}/`, '_blank');
    }
  };

  if (error) {
    return (
      <div className="p-8 lg:p-12 max-w-[1400px] mx-auto space-y-10 animate-fade-in">
        <div className="rounded-xl bg-red-500/10 border border-red-500/20 p-6">
          <p className="text-red-500 font-semibold">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8 lg:p-12 max-w-[1400px] mx-auto space-y-10 animate-fade-in">
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-6">
        <div className="flex flex-col gap-2">
          <h1 className="text-4xl font-bold tracking-tight">My Workspaces</h1>
          <p className="text-text-secondary text-lg">
            Manage and monitor your development environments.
          </p>
        </div>
        <Link
          to="/workspaces/create"
          className="flex items-center justify-center gap-2 rounded-xl bg-primary hover:bg-primary-dark text-white px-8 py-3.5 text-sm font-bold shadow-xl shadow-primary/20 transition-all active:scale-95"
        >
          <span className="material-symbols-outlined text-xl">add_circle</span>
          <span>Create New Workspace</span>
        </Link>
      </div>

      <div className="bg-surface-dark border border-border-dark rounded-2xl p-5 shadow-sm">
        <div className="flex flex-col lg:flex-row gap-6 justify-between items-center">
          <div className="relative w-full lg:max-w-md group">
            <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-text-secondary group-focus-within:text-primary transition-colors">
              <span className="material-symbols-outlined">search</span>
            </div>
            <input
              className="block w-full rounded-xl border-border-dark bg-background-dark text-white pl-11 pr-4 py-3 focus:border-primary focus:ring-primary sm:text-sm outline-none transition-all placeholder:text-text-secondary/50"
              placeholder="Search by name, ID..."
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </div>
          <div className="flex w-full lg:w-auto items-center gap-5">
            <div className="flex items-center gap-3">
              <span className="text-text-secondary text-xs font-bold uppercase tracking-widest whitespace-nowrap">
                Sort by:
              </span>
              <select
                className="bg-background-dark border-border-dark text-white rounded-xl py-2 px-4 text-sm focus:border-primary focus:ring-primary outline-none cursor-pointer"
                value={sortBy}
                onChange={(e) => setSortBy(e.target.value as 'newest' | 'name')}
              >
                <option value="newest">Created: Newest</option>
                <option value="name">Name: A-Z</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="text-center">
            <div className="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-primary mb-4"></div>
            <p className="text-text-secondary">Loading workspaces...</p>
          </div>
        </div>
      ) : sortedWorkspaces.length === 0 ? (
        <div className="text-center py-20">
          <span className="material-symbols-outlined text-6xl text-text-secondary/30 mb-4 block">
            folder_open
          </span>
          <p className="text-text-secondary text-lg mb-6">No workspaces found</p>
          <Link
            to="/workspaces/create"
            className="inline-flex items-center justify-center gap-2 rounded-xl bg-primary hover:bg-primary-dark text-white px-8 py-3.5 text-sm font-bold"
          >
            <span className="material-symbols-outlined text-xl">add_circle</span>
            <span>Create Your First Workspace</span>
          </Link>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-8">
          {sortedWorkspaces.map((ws) => {
            const pinnedServices = ws.services?.filter((s) => s.is_pinned) || [];

            return (
              <div
                key={ws.id}
                className="group bg-surface-dark border border-border-dark rounded-2xl p-6 hover:border-primary/50 hover:shadow-2xl hover:shadow-primary/5 transition-all duration-300 flex flex-col h-full shadow-lg"
              >
                <div className="flex justify-between items-start mb-5">
                  <div className="flex items-start gap-4 flex-1">
                    <div className="p-2.5 rounded-xl bg-primary/10 text-primary transition-colors">
                      <span className="material-symbols-outlined">folder</span>
                    </div>
                    <div className="flex-1">
                      <Link
                        to={`/workspaces/${ws.id}`}
                        className="text-lg font-bold text-white group-hover:text-primary transition-colors block"
                      >
                        {ws.name}
                      </Link>
                      <div className="flex items-center gap-2 mt-1.5">
                        <span className="flex size-2 rounded-full bg-emerald-500"></span>
                        <span className="text-[10px] font-bold uppercase tracking-widest text-emerald-500">
                          Active
                        </span>
                      </div>
                    </div>
                  </div>
                  <button
                    onClick={() => handleDeleteWorkspace(ws.id)}
                    className="text-text-secondary hover:text-red-500 p-2 rounded-lg hover:bg-white/5 transition-all"
                    title="Delete workspace"
                  >
                    <span className="material-symbols-outlined">delete</span>
                  </button>
                </div>
                <p className="text-sm text-text-secondary mb-6 line-clamp-2 leading-relaxed">
                  {ws.description || 'No description'}
                </p>

                {/* Pinned Services Section */}
                {pinnedServices.length > 0 && (
                  <div className="mb-6 space-y-3">
                    <p className="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                      Pinned Services
                    </p>
                    <div className="flex flex-wrap gap-2">
                      {pinnedServices.map((svc) => (
                        <button
                          key={svc.id}
                          onClick={(e) => handleServiceClick(e, svc)}
                          className="flex items-center gap-2 px-3 py-2 rounded-lg bg-background-dark/50 border border-border-dark hover:border-primary/50 hover:bg-background-dark transition-all group/svc"
                          title={svc.agent_type === 'ssh' ? 'View Details' : 'Open Web Access'}
                        >
                          <span
                            className={`material-symbols-outlined text-[18px] ${getServiceColor(svc.agent_type)}`}
                          >
                            {getServiceIcon(svc.agent_type)}
                          </span>
                          <span className="text-xs font-bold text-white group-hover/svc:text-primary transition-colors max-w-[120px] truncate">
                            {svc.name}
                          </span>
                          <span className="material-symbols-outlined text-[14px] text-text-secondary opacity-0 group-hover/svc:opacity-100 -ml-1 transition-all">
                            {svc.agent_type === 'ssh' ? 'chevron_right' : 'open_in_new'}
                          </span>
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                <div className="flex-1 flex items-end mb-4">
                  <span className="text-[10px] font-bold uppercase tracking-widest text-slate-600">
                    Created {new Date(ws.created_at).toLocaleDateString()}
                  </span>
                </div>
                <div className="pt-4 border-t border-border-dark">
                  <Link
                    to={`/workspaces/${ws.id}`}
                    className="inline-flex items-center gap-2 text-primary hover:text-primary-dark text-sm font-semibold transition-colors"
                  >
                    <span>View Services</span>
                    <span className="material-symbols-outlined text-lg">arrow_forward</span>
                  </Link>
                </div>
              </div>
            );
          })}

          <Link
            to="/workspaces/create"
            className="group flex flex-col items-center justify-center gap-5 bg-background-dark/30 border-2 border-dashed border-border-dark rounded-2xl p-10 hover:border-primary hover:bg-primary/5 transition-all duration-300 min-h-[320px]"
          >
            <div className="size-16 rounded-full bg-surface-dark shadow-xl flex items-center justify-center text-primary group-hover:scale-110 transition-transform border border-border-dark group-hover:border-primary">
              <span className="material-symbols-outlined text-4xl">add</span>
            </div>
            <div className="text-center">
              <h3 className="text-lg font-bold text-white group-hover:text-primary mb-1 transition-colors">
                Create New Workspace
              </h3>
              <p className="text-sm text-text-secondary">Launch a new development environment</p>
            </div>
          </Link>
        </div>
      )}
    </div>
  );
};

export default Workspaces;
