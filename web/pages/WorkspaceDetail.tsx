import React, { useState, useEffect } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import {
  workspacesApi,
  servicesApi,
  Workspace,
  Service,
  systemConfigApi,
  SystemConfig,
} from '../services/api';

const WorkspaceDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  const [services, setServices] = useState<Service[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filterQuery, setFilterQuery] = useState('');
  const [systemConfig, setSystemConfig] = useState<SystemConfig | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      if (!id) return;

      try {
        setLoading(true);
        setError(null);

        // Fetch all data in parallel
        const [ws, svcs, config] = await Promise.all([
          workspacesApi.get(id),
          servicesApi.listByWorkspace(id),
          systemConfigApi.get().catch(() => null), // Fail gracefully
        ]);

        setWorkspace(ws);
        setServices(svcs);
        setSystemConfig(config);
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to load workspace';
        setError(message);
        console.error('Error fetching workspace:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [id]);

  const handleDeleteWorkspace = async () => {
    if (
      !id ||
      !window.confirm('Are you sure you want to delete this workspace? This cannot be undone.')
    )
      return;

    try {
      await workspacesApi.delete(id);
      navigate('/workspaces');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to delete workspace';
      setError(message);
    }
  };

  const handleDeleteService = async (serviceId: string) => {
    if (!window.confirm('Are you sure you want to delete this service?')) return;

    try {
      await servicesApi.delete(serviceId);
      setServices(services.filter((s) => s.id !== serviceId));
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to delete service';
      setError(message);
    }
  };

  const filteredServices = services.filter(
    (svc) =>
      svc.name.toLowerCase().includes(filterQuery.toLowerCase()) ||
      svc.id.toLowerCase().includes(filterQuery.toLowerCase()),
  );

  const getAgentTypeIcon = (type?: string) => {
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

  const getAgentTypeColor = (type?: string) => {
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

  const getAgentTypeLabel = (type?: string) => {
    if (!type || typeof type !== 'string') return 'Unknown';
    return type.charAt(0).toUpperCase() + type.slice(1);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="text-center">
          <div className="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-primary mb-4"></div>
          <p className="text-text-secondary">Loading workspace...</p>
        </div>
      </div>
    );
  }

  if (error || !workspace) {
    return (
      <div className="p-8 lg:p-12 max-w-[1400px] mx-auto">
        <div className="rounded-xl bg-red-500/10 border border-red-500/20 p-6">
          <p className="text-red-500 font-semibold">{error || 'Workspace not found'}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8 lg:p-10 max-w-[1400px] mx-auto space-y-8 animate-fade-in">
      {/* Breadcrumbs and Top Info */}
      <div className="flex flex-col gap-6">
        <nav className="flex items-center gap-3 text-xs font-medium">
          <Link to="/" className="text-text-secondary hover:text-text-foreground transition-colors">
            <span className="material-symbols-outlined text-[18px]">home</span>
          </Link>
          <span className="text-text-secondary">/</span>
          <Link
            to="/workspaces"
            className="text-text-secondary hover:text-text-foreground transition-colors"
          >
            Workspaces
          </Link>
          <span className="text-text-secondary">/</span>
          <span className="bg-surface-highlight text-text-foreground px-2 py-0.5 rounded text-[10px] font-mono">
            {workspace.id}
          </span>
        </nav>

        <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-6">
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-4">
              <h1 className="text-4xl font-bold tracking-tight text-text-foreground">
                {workspace.name}
              </h1>
              <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-widest bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 flex items-center gap-1.5">
                <span className="size-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
                Active
              </span>
            </div>
            {workspace.description && (
              <p className="text-text-secondary text-sm mt-2">{workspace.description}</p>
            )}
          </div>
          <div className="flex gap-3">
            <button
              onClick={handleDeleteWorkspace}
              className="flex items-center gap-2 px-5 py-2 rounded-lg bg-red-500/10 border border-red-500/20 text-red-500 text-xs font-bold uppercase tracking-widest hover:bg-red-500/20 transition-all"
            >
              <span className="material-symbols-outlined text-[18px]">delete</span>
              <span>Delete</span>
            </button>
          </div>
        </div>
      </div>

      {/* Tabs and Actions Area */}
      <div className="space-y-6 pt-4">
        {/* Tabs - Only Services Tab per request */}
        <div className="flex items-center gap-8 border-b border-border-dark overflow-x-auto whitespace-nowrap scrollbar-hide">
          <button className="px-2 py-4 text-xs font-bold uppercase tracking-widest text-primary border-b-2 border-primary flex items-center gap-2 transition-all">
            <span className="material-symbols-outlined text-[20px] icon-fill">grid_view</span>
            Services
            <span className="bg-primary/20 text-primary text-[10px] px-2 py-0.5 rounded-full font-bold ml-1">
              {services.length}
            </span>
          </button>
        </div>

        {error && (
          <div className="rounded-xl bg-red-500/10 border border-red-500/20 p-6">
            <p className="text-red-500 font-semibold">{error}</p>
          </div>
        )}

        {/* Action Bar */}
        <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
          <div className="flex items-center gap-3 w-full sm:w-auto">
            <div className="relative w-full sm:w-64 group">
              <span className="material-symbols-outlined absolute left-3 top-2 text-text-secondary group-focus-within:text-primary text-[20px] transition-colors">
                search
              </span>
              <input
                className="w-full pl-10 pr-4 py-2 bg-surface-dark/50 border border-border-dark rounded-lg text-sm text-text-foreground focus:ring-1 focus:ring-primary focus:border-primary outline-none transition-all placeholder:text-text-secondary/30"
                placeholder="Filter services..."
                type="text"
                value={filterQuery}
                onChange={(e) => setFilterQuery(e.target.value)}
              />
            </div>
          </div>
          {id && (
            <Link
              to={`/workspaces/${id}/services/create`}
              className="w-full sm:w-auto flex items-center justify-center gap-2 px-5 py-2 bg-primary text-white rounded-lg text-xs font-bold uppercase tracking-widest hover:bg-primary-dark transition-all shadow-lg shadow-primary/20 active:scale-95"
            >
              <span className="material-symbols-outlined text-[20px]">add_box</span>
              Create Service
            </Link>
          )}
        </div>

        {/* Services Table */}
        {filteredServices.length === 0 ? (
          <div className="text-center py-20">
            <span className="material-symbols-outlined text-6xl text-text-secondary/30 mb-4 block">
              deployed_code
            </span>
            <p className="text-text-secondary text-lg mb-6">
              {services.length === 0 ? 'No services yet' : 'No services matching your search'}
            </p>
            {id && services.length === 0 && (
              <Link
                to={`/workspaces/${id}/services/create`}
                className="inline-flex items-center gap-2 px-5 py-2.5 bg-primary text-white rounded-lg text-xs font-bold uppercase tracking-widest hover:bg-primary-dark transition-all"
              >
                <span className="material-symbols-outlined">add_box</span>
                Create Your First Service
              </Link>
            )}
          </div>
        ) : (
          <div className="bg-surface-dark/40 rounded-xl border border-border-dark overflow-hidden shadow-xl">
            <div className="overflow-x-auto">
              <table className="w-full text-left whitespace-nowrap">
                <thead className="bg-background-dark/30 border-b border-border-dark text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  <tr>
                    <th className="px-6 py-4 font-medium">Service Name</th>
                    <th className="px-6 py-4 font-medium">Status</th>
                    <th className="px-6 py-4 font-medium">Agent Type</th>
                    <th className="px-6 py-4 font-medium">Created</th>
                    <th className="px-6 py-4 font-medium">Access</th>
                    <th className="px-6 py-4 font-medium text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border-dark/30 font-medium text-sm">
                  {filteredServices.map((svc) => (
                    <tr key={svc.id} className="hover:bg-surface-highlight transition-colors group">
                      <td className="px-6 py-5">
                        <div className="flex items-center gap-3">
                          <div className="size-10 rounded-lg bg-surface-dark flex items-center justify-center border border-border-dark">
                            <span
                              className={`material-symbols-outlined text-[20px] ${getAgentTypeColor(svc.agent_type)}`}
                            >
                              {getAgentTypeIcon(svc.agent_type)}
                            </span>
                          </div>
                          <div>
                            <Link
                              to={`/services/${svc.id}`}
                              className="font-bold text-text-foreground text-xs hover:text-primary transition-colors"
                            >
                              {svc.name}
                            </Link>
                            <p className="text-[10px] text-text-secondary opacity-60 mt-0.5 font-mono">
                              {svc.id}
                            </p>
                          </div>
                        </div>
                      </td>
                      <td className="px-6 py-5">
                        <span
                          className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-widest ${svc.status === 'running'
                              ? 'bg-emerald-500/10 text-emerald-500'
                              : 'bg-red-500/10 text-red-500'
                            }`}
                        >
                          <span
                            className={`size-2 rounded-full ${svc.status === 'running' ? 'bg-emerald-500' : 'bg-red-500'}`}
                          ></span>
                          {svc.status}
                        </span>
                      </td>
                      <td className="px-6 py-5">
                        {svc.agent_type ? (
                          <div className="inline-flex items-center gap-2 px-2.5 py-1.5 rounded-lg bg-primary/15 border border-primary/20">
                            <span className="material-symbols-outlined text-[16px] text-primary">
                              {getAgentTypeIcon(svc.agent_type)}
                            </span>
                            <span className="text-[10px] font-bold text-text-foreground uppercase tracking-widest">
                              {getAgentTypeLabel(svc.agent_type)}
                            </span>
                          </div>
                        ) : (
                          <span className="text-[11px] text-text-secondary/50 italic">
                            No template
                          </span>
                        )}
                      </td>
                      <td className="px-6 py-5 text-[11px] text-text-secondary">
                        {new Date(svc.created_at).toLocaleDateString()}
                      </td>
                      <td className="px-6 py-5">
                        {svc.agent_type?.toLowerCase().trim() === 'ssh' ? (
                          <Link
                            to={`/services/${svc.id}`}
                            className="inline-flex items-center gap-2 px-4 py-2 bg-primary/10 hover:bg-primary/20 text-primary text-[10px] font-bold uppercase tracking-widest rounded-lg border border-primary/20 transition-all"
                          >
                            <span className="material-symbols-outlined text-[16px]">terminal</span>
                            View Details
                          </Link>
                        ) : (
                          <button
                            onClick={() => {
                              const url = svc.remote_proxy
                                ? svc.remote_proxy
                                : systemConfig
                                  ? `${svc.agent_id}.${systemConfig.agent_domain}`
                                  : null;

                              if (url) {
                                window.open(`http://${url}/`, '_blank');
                              } else {
                                alert(
                                  'Service URL is not available yet. Please wait for the service to start.',
                                );
                              }
                            }}
                            className="inline-flex items-center gap-2 px-4 py-2 bg-primary/10 hover:bg-primary/20 text-primary text-[10px] font-bold uppercase tracking-widest rounded-lg border border-primary/20 transition-all"
                          >
                            <span className="material-symbols-outlined text-[16px]">
                              open_in_new
                            </span>
                            Open
                          </button>
                        )}
                      </td>
                      <td className="px-6 py-5 text-right">
                        <Link
                          to={`/services/${svc.id}/edit`}
                          className="text-text-secondary hover:text-primary transition-all p-1.5 rounded-lg hover:bg-white/10 inline-block mr-2"
                          title="Edit service"
                        >
                          <span className="material-symbols-outlined text-[20px]">edit</span>
                        </Link>
                        <button
                          onClick={() => handleDeleteService(svc.id)}
                          className="text-text-secondary hover:text-red-500 transition-all p-1.5 rounded-lg hover:bg-white/10"
                          title="Delete service"
                        >
                          <span className="material-symbols-outlined text-[20px]">delete</span>
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {/* Table Footer */}
            <div className="px-6 py-4 border-t border-border-dark flex items-center justify-between bg-background-dark/20">
              <p className="text-[11px] text-text-secondary font-medium">
                Showing {filteredServices.length} of {services.length} services
              </p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default WorkspaceDetail;
