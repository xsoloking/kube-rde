import React, { useState } from 'react';
import { useLocation, Link } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

const Header: React.FC = () => {
  const location = useLocation();
  const { user, logout } = useAuth();
  const [showUserMenu, setShowUserMenu] = useState(false);
  const pathParts = location.pathname.split('/').filter((p) => p);

  const handleLogout = async () => {
    setShowUserMenu(false);
    await logout();
  };

  return (
    <header className="sticky top-0 z-50 w-full border-b border-border-dark bg-background-dark/80 backdrop-blur-md h-16 shrink-0">
      <div className="px-6 h-full flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div className="flex lg:hidden size-8 text-primary items-center justify-center">
            <span className="material-symbols-outlined text-3xl">deployed_code</span>
          </div>
          <nav className="hidden sm:flex items-center gap-2 text-sm">
            <span className="text-text-secondary">Platform</span>
            {pathParts.map((part, i) => {
              // Ensure part is a string
              const partStr = String(part);
              return (
                <React.Fragment key={i}>
                  <span className="text-border-dark text-xs font-bold">/</span>
                  <span
                    className={
                      i === pathParts.length - 1 ? 'text-white font-medium' : 'text-text-secondary'
                    }
                  >
                    {partStr.charAt(0).toUpperCase() + partStr.slice(1)}
                  </span>
                </React.Fragment>
              );
            })}
          </nav>
        </div>

        <div className="flex items-center gap-4">
          <div className="relative hidden md:block w-64 group">
            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-text-secondary group-focus-within:text-primary transition-colors">
              <span className="material-symbols-outlined text-[20px]">search</span>
            </span>
            <input
              className="w-full h-10 pl-10 pr-4 bg-surface-dark border border-border-dark rounded-lg focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all placeholder:text-text-secondary/50 text-sm"
              placeholder="Quick search (⌘K)..."
              type="text"
            />
          </div>
          <button className="relative p-2 text-text-secondary hover:text-white transition-colors rounded-lg hover:bg-white/5">
            <span className="material-symbols-outlined text-[20px]">notifications</span>
            <span className="absolute top-2 right-2 size-2 bg-red-500 rounded-full border-2 border-background-dark"></span>
          </button>
          <Link
            to="/help"
            className="p-2 text-text-secondary hover:text-white transition-colors rounded-lg hover:bg-white/5"
            title="Help Center"
          >
            <span className="material-symbols-outlined text-[20px]">help</span>
          </Link>

          {/* User menu */}
          <div className="relative">
            <button
              onClick={() => setShowUserMenu(!showUserMenu)}
              className="flex items-center gap-2 p-2 text-text-secondary hover:text-white transition-colors rounded-lg hover:bg-white/5 ml-2"
            >
              <div className="h-8 w-8 rounded-full bg-gradient-to-tr from-primary to-purple-500 flex items-center justify-center text-white font-bold text-xs border border-border-dark">
                {user?.username?.substring(0, 2).toUpperCase() || 'U'}
              </div>
              <span className="text-sm font-medium hidden sm:block">
                {user?.username || 'User'}
              </span>
              <span className="material-symbols-outlined text-[20px]">expand_more</span>
            </button>

            {/* User menu dropdown */}
            {showUserMenu && (
              <div className="absolute right-0 mt-2 w-64 bg-surface-dark border border-border-dark rounded-lg shadow-lg overflow-hidden z-10">
                <div className="px-4 py-3 border-b border-border-dark">
                  <p className="text-sm text-white font-medium">{user?.username || 'User'}</p>
                  <p className="text-xs text-text-secondary">{user?.email || 'No email'}</p>
                  {user?.roles && user.roles.length > 0 && (
                    <div className="mt-2 flex gap-1 flex-wrap">
                      {user.roles.map((role) => (
                        <span
                          key={role}
                          className="text-xs bg-primary/20 text-primary px-2 py-1 rounded"
                        >
                          {role}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
                <Link
                  to={`/users/${user?.id}`}
                  onClick={() => setShowUserMenu(false)}
                  className="w-full px-4 py-2.5 text-sm text-white hover:bg-white/5 transition-colors flex items-center gap-2"
                >
                  <span className="material-symbols-outlined text-[18px] text-primary">person</span>
                  Profile Details
                </Link>
                <div className="h-px bg-border-dark"></div>
                <button
                  onClick={handleLogout}
                  className="w-full px-4 py-2.5 text-sm text-red-400 hover:bg-red-500/10 transition-colors text-left flex items-center gap-2"
                >
                  <span className="material-symbols-outlined text-[18px]">logout</span>
                  Sign Out
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
    </header>
  );
};

export default Header;
