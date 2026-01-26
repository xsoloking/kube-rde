import React from 'react';
import { HashRouter, Routes, Route, useLocation } from 'react-router-dom';
import { AuthProvider } from './contexts/AuthContext';
import { ThemeProvider } from './contexts/ThemeContext';
import { ProtectedRoute } from './components/ProtectedRoute';
import Sidebar from './components/Sidebar';
import Header from './components/Header';
import Login from './pages/Login';
import UserManagement from './pages/UserManagement';
import UserEdit from './pages/UserEdit';
import TeamManagement from './pages/TeamManagement';
import TeamQuotaEdit from './pages/TeamQuotaEdit';
import AgentTemplates from './pages/AgentTemplates';
import ResourceManagement from './pages/ResourceManagement';
import AdminWorkspaces from './pages/AdminWorkspaces';
import AuditLogs from './pages/AuditLogs';
import Help from './pages/Help';
import Dashboard from './pages/Dashboard';
import Workspaces from './pages/Workspaces';
import WorkspaceCreate from './pages/WorkspaceCreate';
import WorkspaceDetail from './pages/WorkspaceDetail';
import ServiceDetail from './pages/ServiceDetail';
import ServiceCreate from './pages/ServiceCreate';
import ServiceEdit from './pages/ServiceEdit';

const AppLayout: React.FC = () => {
  const location = useLocation();
  const isLoginPage = location.pathname === '/login';

  if (isLoginPage) {
    return (
      <Routes>
        <Route path="/login" element={<Login />} />
      </Routes>
    );
  }

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-background-dark font-body text-text-foreground selection:bg-primary/30">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <Header />
        <main className="flex-1 overflow-y-auto overflow-x-hidden">
          <Routes>
            <Route
              path="/"
              element={
                <ProtectedRoute>
                  <Dashboard />
                </ProtectedRoute>
              }
            />
            <Route
              path="/users"
              element={
                <ProtectedRoute requiredRole="admin">
                  <UserManagement />
                </ProtectedRoute>
              }
            />
            <Route
              path="/users/:id"
              element={
                <ProtectedRoute>
                  <UserEdit />
                </ProtectedRoute>
              }
            />
            <Route
              path="/teams"
              element={
                <ProtectedRoute requiredRole="admin">
                  <TeamManagement />
                </ProtectedRoute>
              }
            />
            <Route
              path="/teams/:id/quota"
              element={
                <ProtectedRoute requiredRole="admin">
                  <TeamQuotaEdit />
                </ProtectedRoute>
              }
            />
            <Route
              path="/agent-templates"
              element={
                <ProtectedRoute requiredRole="admin">
                  <AgentTemplates />
                </ProtectedRoute>
              }
            />
            <Route
              path="/resource-management"
              element={
                <ProtectedRoute requiredRole="admin">
                  <ResourceManagement />
                </ProtectedRoute>
              }
            />
            <Route
              path="/admin/workspaces"
              element={
                <ProtectedRoute requiredRole="admin">
                  <AdminWorkspaces />
                </ProtectedRoute>
              }
            />
            <Route
              path="/admin/audit"
              element={
                <ProtectedRoute requiredRole="admin">
                  <AuditLogs />
                </ProtectedRoute>
              }
            />
            <Route
              path="/workspaces"
              element={
                <ProtectedRoute>
                  <Workspaces />
                </ProtectedRoute>
              }
            />
            <Route
              path="/help"
              element={
                <ProtectedRoute>
                  <Help />
                </ProtectedRoute>
              }
            />
            <Route
              path="/workspaces/create"
              element={
                <ProtectedRoute>
                  <WorkspaceCreate />
                </ProtectedRoute>
              }
            />
            <Route
              path="/workspaces/:id"
              element={
                <ProtectedRoute>
                  <WorkspaceDetail />
                </ProtectedRoute>
              }
            />
            <Route
              path="/workspaces/:workspaceId/services/create"
              element={
                <ProtectedRoute>
                  <ServiceCreate />
                </ProtectedRoute>
              }
            />
            <Route
              path="/services/:id"
              element={
                <ProtectedRoute>
                  <ServiceDetail />
                </ProtectedRoute>
              }
            />
            <Route
              path="/services/:id/edit"
              element={
                <ProtectedRoute>
                  <ServiceEdit />
                </ProtectedRoute>
              }
            />
          </Routes>
        </main>
      </div>
    </div>
  );
};

const App: React.FC = () => {
  return (
    <ThemeProvider>
      <AuthProvider>
        <HashRouter>
          <AppLayout />
        </HashRouter>
      </AuthProvider>
    </ThemeProvider>
  );
};

export default App;
