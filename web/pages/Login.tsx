import React, { useEffect } from 'react';

const Login: React.FC = () => {
  // Clear any stale return_url when login page is mounted
  // This prevents redirect loops after logout
  useEffect(() => {
    // Clear sessionStorage on mount to ensure clean state
    sessionStorage.removeItem('return_url');
  }, []);

  const handleLogin = () => {
    // Get return URL from sessionStorage (set by ProtectedRoute)
    let returnUrl = sessionStorage.getItem('return_url') || '/';

    // Prevent redirect loop: ignore if return_url is login page
    if (returnUrl === '/login' || returnUrl === '/#/login') {
      returnUrl = '/';
    }

    // Clear sessionStorage to prevent stale return_url
    sessionStorage.removeItem('return_url');

    // Redirect to OAuth login endpoint which will start the authentication flow
    window.location.href = '/auth/login?return_url=' + encodeURIComponent(returnUrl);
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-background-dark via-surface-dark to-background-dark">
      <div className="max-w-md w-full mx-4">
        {/* Logo and Title */}
        <div className="text-center mb-8">
          <div className="flex items-center justify-center mb-6">
            <img src="/icon.png" alt="KubeRDE" className="size-28 rounded-2xl shadow-xl shadow-primary/20" />
          </div>
          <h1 className="text-4xl font-bold text-white mb-2">KubeRDE</h1>
          <p className="text-text-secondary text-lg">Remote Development Environment</p>
        </div>

        {/* Login Card */}
        <div className="bg-surface-dark border border-border-dark rounded-2xl shadow-2xl p-8">
          <div className="mb-6">
            <h2 className="text-2xl font-bold text-white mb-2">Welcome Back</h2>
            <p className="text-text-secondary">Sign in to access your development workspace</p>
          </div>

          <button
            onClick={handleLogin}
            className="w-full flex items-center justify-center gap-3 bg-primary hover:bg-primary-dark text-white px-6 py-4 rounded-xl font-bold text-lg shadow-xl shadow-primary/20 transition-all active:scale-95"
          >
            <span className="material-symbols-outlined text-2xl">login</span>
            <span>Sign in with SSO</span>
          </button>

          <div className="mt-6 pt-6 border-t border-border-dark">
            <p className="text-center text-sm text-text-secondary">
              Powered by Keycloak Single Sign-On
            </p>
          </div>
        </div>

        {/* Footer */}
        <div className="mt-8 text-center text-sm text-text-secondary">
          <p>&copy; 2025 KubeRDE. All rights reserved.</p>
        </div>
      </div>
    </div>
  );
};

export default Login;
