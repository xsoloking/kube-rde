import React from 'react';
import { useLocation, Navigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

interface ProtectedRouteProps {
  children: React.ReactNode;
  requiredRole?: string;
}

export const ProtectedRoute: React.FC<ProtectedRouteProps> = ({ children, requiredRole }) => {
  const { user, loading, isAuthenticated } = useAuth();
  const location = useLocation();

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto"></div>
          <p className="mt-4 text-gray-400">Loading...</p>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    // Redirect to static login page, not OAuth endpoint
    // Save return URL in sessionStorage for after login
    const returnUrl = location.pathname + location.search;
    // Don't save login page as return URL to avoid redirect loops
    if (!returnUrl.includes('/login')) {
      sessionStorage.setItem('return_url', returnUrl);
    }
    return <Navigate to="/login" replace />;
  }

  // Check role permissions if required
  const userRoles = user?.roles || [];
  if (requiredRole && user && !userRoles.includes(requiredRole)) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-center">
          <h1 className="text-3xl font-bold text-red-500">Access Denied</h1>
          <p className="mt-4 text-gray-400">You don't have permission to access this page.</p>
          <p className="mt-2 text-sm text-gray-500">
            Required role: <span className="font-mono text-blue-400">{requiredRole}</span>
          </p>
          <a href="/" className="mt-6 inline-block text-blue-400 hover:text-blue-300">
            Go back to dashboard
          </a>
        </div>
      </div>
    );
  }

  return <>{children}</>;
};
