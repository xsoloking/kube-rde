import React, { createContext, useContext, useState, useEffect } from 'react';
import { authApi, User } from '../services/api';

interface AuthContextType {
  user: User | null;
  loading: boolean;
  isAuthenticated: boolean;
  login: () => void;
  logout: () => Promise<void>;
  refreshToken: () => Promise<void>;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const isAuthenticated = user !== null;

  // Load current user on mount
  useEffect(() => {
    loadUser();
  }, []);

  // Token auto-refresh interval
  useEffect(() => {
    if (!isAuthenticated) return;

    // Refresh token every 23 hours 55 minutes (5 minutes before 24-hour expiry)
    const refreshInterval = setInterval(
      () => {
        refreshToken().catch((err) => {
          console.error('Token refresh failed:', err);
          // On failure, redirect to login page
          setUser(null);
          // Only save return_url if not on login page
          const currentPath = window.location.hash.replace('#', '') || window.location.pathname;
          if (!currentPath.includes('/login')) {
            sessionStorage.setItem('return_url', currentPath);
          }
          window.location.href = '/#/login';
        });
      },
      (23 * 60 + 55) * 60 * 1000,
    ); // 23h55m

    return () => clearInterval(refreshInterval);
  }, [isAuthenticated]);

  const loadUser = async () => {
    try {
      const userData = await authApi.getCurrentUser();
      setUser(userData);
    } catch (error) {
      console.error('Failed to load user:', error);
      setUser(null);
    } finally {
      setLoading(false);
    }
  };

  const login = () => {
    window.location.href = '/auth/login?return_url=' + encodeURIComponent(window.location.href);
  };

  const logout = async () => {
    try {
      // Clear any stored return URLs to prevent redirect loops
      sessionStorage.removeItem('return_url');

      const response = await authApi.logout();
      setUser(null);
      // Redirect to Keycloak logout to clear SSO session
      // Keycloak will redirect back to /login (static login page) after logout
      if (response && response.redirect_to) {
        window.location.href = response.redirect_to;
      } else {
        window.location.href = '/#/login';
      }
    } catch (error) {
      console.error('Logout failed:', error);
      // Clear return_url even on error
      sessionStorage.removeItem('return_url');
      setUser(null);
      window.location.href = '/#/login';
    }
  };

  const refreshToken = async () => {
    try {
      await authApi.refresh();
      console.log('Token refreshed successfully');
    } catch (error) {
      console.error('Failed to refresh token:', error);
      throw error;
    }
  };

  const refreshUser = async () => {
    try {
      const userData = await authApi.getCurrentUser();
      setUser(userData);
    } catch (error) {
      console.error('Failed to refresh user data:', error);
      throw error;
    }
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        isAuthenticated,
        login,
        logout,
        refreshToken,
        refreshUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
};
