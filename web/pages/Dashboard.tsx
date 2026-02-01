import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { workspacesApi, userQuotaApi, UserQuota, Workspace } from '../services/api';

const StatCard: React.FC<{
  label: string;
  value: string;
  sub?: string;
  icon: string;
  color?: string;
}> = ({ label, value, sub, icon, color = 'text-primary' }) => (
  <div className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm">
    <div className="flex justify-between items-start mb-4">
      <p className="text-xs font-bold text-text-secondary uppercase tracking-wider">{label}</p>
      <span className={`material-symbols-outlined ${color}`}>{icon}</span>
    </div>
    <div className="flex flex-col gap-1">
      <p className="text-3xl font-bold text-text-foreground">{value}</p>
      {sub && <p className="text-xs text-text-secondary font-medium">{sub}</p>}
    </div>
  </div>
);

const UsageBar: React.FC<{
  label: string;
  used: number;
  total: number;
  unit: string;
  color: string;
  icon: string;
}> = ({ label, used, total, unit, color, icon }) => {
  const percentage = total > 0 ? Math.min(100, (used / total) * 100) : 0;

  return (
    <div className="bg-surface-dark rounded-xl p-5 border border-border-dark flex flex-col gap-3">
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-2">
          <span className="material-symbols-outlined text-text-secondary text-sm">{icon}</span>
          <span className="text-text-foreground font-medium text-sm">{label}</span>
        </div>
        <span className="text-xs font-bold text-text-foreground">
          {used} / {total} {unit}
        </span>
      </div>
      <div className="h-2 w-full bg-background-dark rounded-full overflow-hidden">
        <div
          className="h-full rounded-full transition-all duration-1000"
          style={{ backgroundColor: color, width: `${percentage}%` }}
        ></div>
      </div>
      <p className="text-[10px] text-text-secondary text-right">
        {percentage.toFixed(1)}% Allocated
      </p>
    </div>
  );
};

const Dashboard: React.FC = () => {
  const { user } = useAuth();
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [quota, setQuota] = useState<UserQuota | null>(null);
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState({
    totalWorkspaces: 0,
    activeServices: 0,
    totalServices: 0,
    allocatedCPU: 0,
    allocatedMemory: 0,
    allocatedStorage: 0,
    allocatedGPU: 0,
  });

  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoading(true);
        const [wsList, quotaData] = await Promise.all([
          workspacesApi.list(),
          user?.id ? userQuotaApi.get(user.id) : Promise.resolve(null),
        ]);

        setWorkspaces(wsList);
        setQuota(quotaData);

        // Calculate stats
        let activeSvcs = 0;
        let totalSvcs = 0;
        let cpu = 0;
        let mem = 0;
        let storage = 0;
        let gpu = 0;

        wsList.forEach((ws) => {
          // Parse storage size
          const size = parseInt(ws.storage_size) || 0;
          storage += size;

          if (ws.services) {
            totalSvcs += ws.services.length;
            ws.services.forEach((svc) => {
              if (svc.status === 'running') {
                activeSvcs++;
              }

              // CPU Allocation (using requests/limits logic from creation)
              // Assuming limit is what counts for quota
              // Default limit in backend is 500m (0.5) if not specified, or user value
              // Here we try to parse what's stored or estimate
              // Ideally backend should return allocated resources, but we can sum up from service definitions
              // If cpu_cores is set, use it. Else assume default 0.5
              // Actually models.Service has CPUCores string (e.g. "4")
              // TODO: Align this with backend logic. For now, sum up declared CPUCores.
              // Note: stopped services still consume quota in some models, but usually only running ones consume compute.
              // However, typically Quota is about Provisioned Capacity (Requests/Limits).
              // Let's count all services for allocation.

              if (svc.cpu_cores) {
                cpu += parseFloat(String(svc.cpu_cores));
              } else {
                cpu += 0.5; // Default 500m
              }

              if (svc.memory_gib) {
                mem += parseFloat(String(svc.memory_gib));
              } else {
                mem += 0.5; // Default 512Mi (~0.5Gi)
              }

              if (svc.gpu_count) {
                gpu += svc.gpu_count;
              }
            });
          }
        });

        setStats({
          totalWorkspaces: wsList.length,
          activeServices: activeSvcs,
          totalServices: totalSvcs,
          allocatedCPU: cpu,
          allocatedMemory: mem,
          allocatedStorage: storage,
          allocatedGPU: gpu,
        });
      } catch (err) {
        console.error('Failed to fetch dashboard data:', err);
      } finally {
        setLoading(false);
      }
    };

    if (user) {
      fetchData();
    }
  }, [user]);

  // Sort recent workspaces
  const recentWorkspaces = [...workspaces]
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
    .slice(0, 5);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  return (
    <div className="p-8 lg:p-12 max-w-[1400px] mx-auto space-y-10 animate-fade-in">
      <div className="flex flex-col gap-2">
        <h2 className="text-4xl font-bold tracking-tight text-text-foreground">Dashboard</h2>
        <p className="text-text-secondary text-lg">
          Overview of your development environments and resource allocation.
        </p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-start">
        {/* Left Column: Stats & Allocation */}
        <div className="lg:col-span-8 flex flex-col gap-8">
          {/* Top Stats */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
            <StatCard
              label="Total Workspaces"
              value={stats.totalWorkspaces.toString()}
              icon="folder_open"
              color="text-blue-500"
            />
            <StatCard
              label="Active Services"
              value={stats.activeServices.toString()}
              sub={`out of ${stats.totalServices} total`}
              icon="dns"
              color="text-emerald-500"
            />
          </div>

          {/* Resource Allocation Detail */}
          <div className="bg-surface-dark/30 rounded-2xl border border-border-dark p-6">
            <h3 className="text-xl font-bold text-text-foreground mb-6">Resource Allocation</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <UsageBar
                label="CPU Cores"
                used={stats.allocatedCPU}
                total={quota?.cpu_cores || 0}
                unit="Cores"
                color="#1a79ff"
                icon="memory"
              />
              <UsageBar
                label="Memory (RAM)"
                used={stats.allocatedMemory}
                total={quota?.memory_gi || 0}
                unit="GiB"
                color="#facc15"
                icon="developer_board"
              />
              <UsageBar
                label="Total Storage"
                used={stats.allocatedStorage}
                total={quota?.storage_quota?.reduce((acc, item) => acc + item.limit_gi, 0) || 0}
                unit="GiB"
                color="#34d399"
                icon="hard_drive"
              />
              <UsageBar
                label="GPU Units"
                used={stats.allocatedGPU}
                total={quota?.gpu_quota?.reduce((acc, item) => acc + item.limit, 0) || 0}
                unit="Units"
                color="#f472b6"
                icon="videogame_asset"
              />
            </div>
          </div>
        </div>

        {/* Right Column: Recent Workspaces */}
        <div className="lg:col-span-4 flex flex-col gap-6 h-full">
          <div className="flex items-center justify-between">
            <h3 className="text-xl font-bold text-text-foreground">Recent Workspaces</h3>
            <Link
              to="/workspaces"
              className="text-xs font-bold text-primary hover:text-primary-dark uppercase tracking-widest transition-colors"
            >
              View All
            </Link>
          </div>
          <div className="bg-surface-dark border border-border-dark rounded-xl overflow-hidden flex flex-col h-full">
            {recentWorkspaces.length === 0 ? (
              <div className="p-8 text-center text-text-secondary flex-1 flex flex-col items-center justify-center">
                <span className="material-symbols-outlined text-4xl mb-2 opacity-50">
                  folder_off
                </span>
                <p className="text-sm">No workspaces found</p>
              </div>
            ) : (
              <div className="divide-y divide-border-dark/30 flex-1 overflow-y-auto max-h-[600px] scrollbar-hide">
                {recentWorkspaces.map((ws) => (
                  <Link
                    key={ws.id}
                    to={`/workspaces/${ws.id}`}
                    className="flex items-center justify-between p-4 hover:bg-surface-highlight transition-colors group"
                  >
                    <div className="flex items-center gap-3 min-w-0">
                      <div className="p-2 rounded-lg bg-primary/10 text-primary group-hover:bg-primary group-hover:text-white transition-colors shrink-0">
                        <span className="material-symbols-outlined text-[20px]">folder</span>
                      </div>
                      <div className="min-w-0">
                        <p className="text-sm font-bold text-text-foreground group-hover:text-primary transition-colors truncate">
                          {ws.name}
                        </p>
                        <p className="text-[10px] text-text-secondary uppercase tracking-wider truncate">
                          {ws.storage_class}
                        </p>
                      </div>
                    </div>
                    <span className="material-symbols-outlined text-text-secondary group-hover:text-text-foreground text-[18px] shrink-0">
                      arrow_forward
                    </span>
                  </Link>
                ))}
              </div>
            )}
            {recentWorkspaces.length > 0 && (
              <Link
                to="/workspaces/create"
                className="p-4 text-center text-xs font-bold text-text-secondary hover:text-text-foreground hover:bg-surface-highlight transition-colors border-t border-border-dark/50 uppercase tracking-widest bg-background-dark/30 shrink-0"
              >
                + Create New Workspace
              </Link>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
