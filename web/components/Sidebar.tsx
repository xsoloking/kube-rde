import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

const NavItem: React.FC<{ to: string; icon: string; label: string }> = ({ to, icon, label }) => {
  const location = useLocation();
  const isActive = location.pathname === to || (to !== '/' && location.pathname.startsWith(to));

  return (
    <Link
      to={to}
      className={`flex items-center gap-3 px-4 py-3 rounded-lg transition-all duration-200 group ${
        isActive
          ? 'bg-surface-highlight text-text-foreground shadow-sm border border-border-dark'
          : 'text-text-secondary hover:bg-surface-highlight hover:text-text-foreground'
      }`}
    >
      <span
        className={`material-symbols-outlined ${isActive ? 'text-primary icon-fill' : 'group-hover:text-primary'}`}
      >
        {icon}
      </span>
      <span className="text-sm font-medium">{label}</span>
    </Link>
  );
};

const Sidebar: React.FC = () => {
  const { user } = useAuth();

  // Check if user has admin role
  const isAdmin = user?.roles?.includes('admin') || false;

  return (
    <aside className="w-64 h-full hidden lg:flex flex-col border-r border-border-dark bg-background-dark shrink-0">
      <div className="p-6 flex items-center gap-3">
        <div className="size-10 rounded-full bg-primary flex items-center justify-center shadow-lg shadow-primary/20">
          <span className="material-symbols-outlined text-text-foreground text-2xl">
            deployed_code
          </span>
        </div>
        <div className="flex flex-col">
          <h1 className="text-text-foreground text-lg font-bold leading-none tracking-tight">
            KubeRDE
          </h1>
          <p className="text-text-secondary text-[10px] font-normal uppercase tracking-widest mt-1">
            AI Platform
          </p>
        </div>
      </div>

      <nav className="flex-1 px-4 space-y-1 overflow-y-auto py-4">
        <NavItem to="/" icon="donut_small" label="Dashboard" />
        <NavItem to="/workspaces" icon="dns" label="Workspaces" />

        {/* Administration section - only visible to admin users */}
        {isAdmin && (
          <div className="pt-8 pb-2">
            <p className="px-4 text-[10px] font-bold uppercase tracking-widest text-text-secondary opacity-40 mb-2">
              Administration
            </p>
            <div className="h-px bg-border-dark mx-2 mb-2 opacity-50"></div>
            <NavItem to="/admin/workspaces" icon="folder_managed" label="All Workspaces" />
            <NavItem to="/users" icon="group" label="Users" />
            <NavItem to="/teams" icon="groups" label="Teams" />
            <NavItem to="/agent-templates" icon="category" label="Agent Templates" />
            <NavItem to="/resource-management" icon="storage" label="Resource Management" />
            <NavItem to="/admin/audit" icon="history_edu" label="Audit Logs" />
          </div>
        )}
      </nav>

      <div className="p-4 space-y-4">
        {/* User Card - Click to view profile */}
        <Link
          to={`/users/${user?.id}`}
          className="flex items-center gap-3 p-3 rounded-2xl bg-surface-dark border border-border-dark hover:bg-surface-highlight hover:border-primary/30 transition-all cursor-pointer select-none group"
          title="View Profile"
        >
          <div className="size-10 rounded-full bg-gradient-to-tr from-primary to-purple-500 flex items-center justify-center text-text-foreground font-bold text-xs shrink-0 shadow-lg shadow-black/20">
            {user?.username?.substring(0, 2).toUpperCase() || 'U'}
          </div>
          <div className="flex-1 flex flex-col overflow-hidden">
            <p className="text-text-foreground text-sm font-bold truncate">
              {user?.username || 'User'}
            </p>
            <p className="text-text-secondary text-[11px] truncate opacity-80">
              {user?.email || 'No email'}
            </p>
          </div>
          <span className="material-symbols-outlined text-text-secondary group-hover:text-primary transition-colors">
            arrow_forward
          </span>
        </Link>
      </div>
    </aside>
  );
};

export default Sidebar;
