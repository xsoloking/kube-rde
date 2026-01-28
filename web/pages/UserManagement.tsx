import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { usersApi, User as ApiUser, CreateUserRequest } from '../services/api';
import { useAuth } from '../contexts/AuthContext';

const UserManagement: React.FC = () => {
  const { user: currentUser } = useAuth();
  const [users, setUsers] = useState<ApiUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [roleFilter, setRoleFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [creating, setCreating] = useState(false);

  // Pagination state
  const [currentPage, setCurrentPage] = useState(1);
  const itemsPerPage = 10;

  // Selection state
  const [selectedUsers, setSelectedUsers] = useState<string[]>([]);

  // Create user form state
  const [newUser, setNewUser] = useState<CreateUserRequest>({
    username: '',
    email: '',
    password: '',
    roles: ['developer'],
    enabled: true,
  });

  // Fetch users on mount
  useEffect(() => {
    loadUsers();
  }, []);

  // Reset page and selection when filters change
  useEffect(() => {
    setCurrentPage(1);
    setSelectedUsers([]);
  }, [search, roleFilter, statusFilter]);

  const loadUsers = async () => {
    try {
      setLoading(true);
      setError(null);
      // Fetch up to 1000 users for client-side pagination
      const data = await usersApi.list(0, 1000);
      setUsers(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load users';
      setError(message);
      console.error('Failed to load users:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleSelectAll = (e: React.ChangeEvent<HTMLInputElement>) => {
    const pageUserIds = paginatedUsers.map((u) => u.id);
    if (e.target.checked) {
      // Add current page users to selection (avoiding duplicates)
      const newSelection = Array.from(new Set([...selectedUsers, ...pageUserIds]));
      setSelectedUsers(newSelection);
    } else {
      // Remove current page users from selection
      setSelectedUsers(selectedUsers.filter((id) => !pageUserIds.includes(id)));
    }
  };

  const handleSelectUser = (userId: string) => {
    if (selectedUsers.includes(userId)) {
      setSelectedUsers(selectedUsers.filter((id) => id !== userId));
    } else {
      setSelectedUsers([...selectedUsers, userId]);
    }
  };

  const handleBulkDelete = async () => {
    if (selectedUsers.length === 0) return;

    // Check if current user is in the selection
    if (currentUser && selectedUsers.includes(currentUser.id)) {
      alert('You cannot delete your own account. Please deselect yourself from the list.');
      return;
    }

    // Show confirmation with cascade deletion warning
    const confirmMessage = `⚠️ WARNING: This will permanently delete ${selectedUsers.length} user(s) and ALL their associated data:

• All workspaces owned by these users
• All services in those workspaces
• All agents and containers
• All SSH keys
• All quota settings

This action CANNOT be undone!

Are you sure you want to continue?`;

    if (!confirm(confirmMessage)) {
      return;
    }

    try {
      // Execute deletions in parallel
      await Promise.all(selectedUsers.map((id) => usersApi.delete(id)));
      setSelectedUsers([]);
      await loadUsers();
      alert(`Successfully deleted ${selectedUsers.length} user(s) and their associated data`);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to delete some users: ' + message);
    }
  };

  const handleDeleteUser = async (userId: string) => {
    // Prevent self-deletion
    if (currentUser && userId === currentUser.id) {
      alert('You cannot delete your own account');
      return;
    }

    // Find user to get username for confirmation
    const user = users.find((u) => u.id === userId);
    const username = user?.username || 'this user';

    // Show confirmation with cascade deletion warning
    const confirmMessage = `⚠️ WARNING: This will permanently delete user "${username}" and ALL associated data:

• All workspaces owned by this user
• All services in those workspaces
• All agents and containers
• All SSH keys
• All quota settings

This action CANNOT be undone!

Type "DELETE" to confirm:`;

    const confirmation = prompt(confirmMessage);
    if (confirmation !== 'DELETE') {
      return;
    }

    try {
      await usersApi.delete(userId);
      await loadUsers();
      alert('User and all associated data deleted successfully');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to delete user: ' + message);
    }
  };

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!newUser.username || !newUser.email || !newUser.password) {
      alert('Please fill in all required fields');
      return;
    }

    try {
      setCreating(true);
      await usersApi.create(newUser);
      setShowCreateModal(false);
      setNewUser({
        username: '',
        email: '',
        password: '',
        roles: ['developer'],
        enabled: true,
      });
      await loadUsers();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to create user: ' + message);
    } finally {
      setCreating(false);
    }
  };

  const toggleRole = (role: string) => {
    setNewUser({
      ...newUser,
      roles: newUser.roles.includes(role)
        ? newUser.roles.filter((r) => r !== role)
        : [...newUser.roles, role],
    });
  };

  // Filter users based on search and filters
  const filteredUsers = users.filter((user) => {
    const matchesSearch =
      !search ||
      user.username.toLowerCase().includes(search.toLowerCase()) ||
      (user.email && user.email.toLowerCase().includes(search.toLowerCase()));

    const matchesRole = !roleFilter || user.roles.includes(roleFilter);

    const matchesStatus =
      !statusFilter ||
      (statusFilter === 'active' && user.enabled) ||
      (statusFilter === 'inactive' && !user.enabled);

    return matchesSearch && matchesRole && matchesStatus;
  });

  // Calculate pagination
  const totalPages = Math.ceil(filteredUsers.length / itemsPerPage);
  const startIndex = (currentPage - 1) * itemsPerPage;
  const paginatedUsers = filteredUsers.slice(startIndex, startIndex + itemsPerPage);

  const handlePageChange = (page: number) => {
    if (page >= 1 && page <= totalPages) {
      setCurrentPage(page);
    }
  };

  return (
    <div className="p-8 lg:p-12 max-w-[1400px] mx-auto space-y-8 animate-fade-in">
      {/* ... Header and Stats ... */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-6">
        <div className="flex flex-col gap-2">
          <h2 className="text-4xl font-bold tracking-tight">User Management</h2>
          <p className="text-text-secondary max-w-2xl text-lg">
            Manage user access, roles, and monitor resource limits across your organization.
          </p>
        </div>
        <button
          onClick={() => setShowCreateModal(true)}
          className="flex items-center gap-2 bg-primary hover:bg-primary-dark text-white px-6 py-3 rounded-xl font-bold shadow-xl shadow-primary/20 transition-all active:scale-95"
        >
          <span className="material-symbols-outlined text-[22px]">add</span>
          <span>Add User</span>
        </button>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {[
          {
            label: 'Total Users',
            val: loading ? '-' : filteredUsers.length.toString(),
            icon: 'group',
            col: 'text-primary',
          },
          {
            label: 'Active Users',
            val: loading ? '-' : filteredUsers.filter((u) => u.enabled).length.toString(),
            icon: 'verified_user',
            col: 'text-emerald-500',
          },
          {
            label: 'Admins',
            val: loading
              ? '-'
              : filteredUsers.filter((u) => u.roles.includes('admin')).length.toString(),
            icon: 'admin_panel_settings',
            col: 'text-purple-500',
          },
          {
            label: 'Developers',
            val: loading
              ? '-'
              : filteredUsers.filter((u) => u.roles.includes('developer')).length.toString(),
            icon: 'code',
            col: 'text-blue-500',
          },
        ].map((s, i) => (
          <div
            key={i}
            className="bg-surface-dark p-6 rounded-xl border border-border-dark shadow-sm"
          >
            <div className="flex justify-between items-start mb-2">
              <p className="text-xs font-bold text-text-secondary uppercase tracking-widest">
                {s.label}
              </p>
              <span className={`material-symbols-outlined ${s.col} text-[20px]`}>{s.icon}</span>
            </div>
            <p className="text-3xl font-bold">{s.val}</p>
          </div>
        ))}
      </div>

      {/* ... Filters ... */}
      <div className="flex flex-col lg:flex-row gap-4 justify-between items-stretch lg:items-center bg-surface-dark p-4 rounded-xl border border-border-dark">
        <div className="flex flex-1 w-full lg:w-auto gap-4 flex-col sm:flex-row">
          <div className="relative flex-1 max-w-md group">
            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-text-secondary group-focus-within:text-primary transition-colors">
              <span className="material-symbols-outlined">search</span>
            </span>
            <input
              className="w-full h-11 pl-10 pr-4 bg-background-dark border border-border-dark rounded-xl focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all placeholder:text-text-secondary/50 text-sm"
              placeholder="Search by name or email..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              type="text"
            />
          </div>
          <div className="flex gap-2">
            <select
              value={roleFilter}
              onChange={(e) => setRoleFilter(e.target.value)}
              className="h-11 pl-4 pr-10 bg-background-dark border border-border-dark rounded-xl appearance-none text-sm focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none cursor-pointer min-w-[140px]"
            >
              <option value="">All Roles</option>
              <option value="admin">Admin</option>
              <option value="developer">Developer</option>
            </select>
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="h-11 pl-4 pr-10 bg-background-dark border border-border-dark rounded-xl appearance-none text-sm focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none cursor-pointer min-w-[140px]"
            >
              <option value="">All Status</option>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
            </select>
          </div>
        </div>
        <div className="flex items-center gap-3">
          {selectedUsers.length > 0 && (
            <button
              onClick={handleBulkDelete}
              className="flex items-center gap-2 px-4 h-11 text-sm font-bold text-red-500 bg-red-500/10 hover:bg-red-500/20 border border-red-500/20 rounded-xl transition-all"
            >
              <span className="material-symbols-outlined text-[20px]">delete</span>
              <span>Delete ({selectedUsers.length})</span>
            </button>
          )}
          <button className="flex items-center gap-2 px-6 h-11 text-sm font-bold text-text-foreground bg-background-dark hover:bg-surface-highlight border border-border-dark rounded-xl transition-all">
            <span className="material-symbols-outlined text-[20px]">filter_list</span>
            <span>Filters</span>
          </button>
        </div>
      </div>

      <div className="w-full overflow-hidden rounded-2xl border border-border-dark bg-surface-dark shadow-xl">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-background-dark/50 border-b border-border-dark">
                <th className="p-5 w-12 text-center">
                  <input
                    className="w-4 h-4 rounded border-border-dark bg-transparent text-primary focus:ring-primary cursor-pointer"
                    type="checkbox"
                    checked={
                      paginatedUsers.length > 0 &&
                      paginatedUsers.every((u) => selectedUsers.includes(u.id))
                    }
                    onChange={handleSelectAll}
                  />
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  User
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Role
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Created
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary w-1/4">
                  Resource Usage
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Status
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary text-right">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border-dark font-body text-sm">
              {loading && (
                <tr>
                  <td colSpan={7} className="p-8 text-center text-text-secondary">
                    <div className="flex items-center justify-center gap-2">
                      <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary"></div>
                      <span>Loading users...</span>
                    </div>
                  </td>
                </tr>
              )}
              {error && (
                <tr>
                  <td colSpan={7} className="p-8 text-center text-red-500">
                    <span className="material-symbols-outlined text-[24px] block mb-2">
                      error_outline
                    </span>
                    {error}
                  </td>
                </tr>
              )}
              {!loading && !error && filteredUsers.length === 0 && (
                <tr>
                  <td colSpan={7} className="p-8 text-center text-text-secondary">
                    <span className="material-symbols-outlined text-[32px] block mb-2 opacity-50">
                      group
                    </span>
                    No users found
                  </td>
                </tr>
              )}
              {paginatedUsers.map((user) => (
                <tr key={user.id} className="group hover:bg-surface-highlight transition-colors">
                  <td className="p-5 text-center">
                    <input
                      className="w-4 h-4 rounded border-border-dark bg-transparent text-primary focus:ring-primary cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                      type="checkbox"
                      checked={selectedUsers.includes(user.id)}
                      onChange={() => handleSelectUser(user.id)}
                      disabled={currentUser?.id === user.id}
                      title={
                        currentUser?.id === user.id
                          ? 'You cannot select your own account for deletion'
                          : ''
                      }
                    />
                  </td>
                  <td className="p-5">
                    <div className="flex items-center gap-3">
                      <div className="size-10 rounded-full bg-gradient-to-br from-primary/30 to-primary/10 border border-border-dark flex items-center justify-center">
                        <span className="material-symbols-outlined text-primary">
                          account_circle
                        </span>
                      </div>
                      <div className="flex flex-col">
                        <Link
                          to={`/users/${user.id}`}
                          className="font-bold text-text-foreground hover:text-primary transition-colors"
                        >
                          {user.username}
                        </Link>
                        <span className="text-xs text-text-secondary">{user.email}</span>
                      </div>
                    </div>
                  </td>
                  <td className="p-5">
                    <div className="flex gap-1 flex-wrap">
                      {user.roles.map((role) => (
                        <span
                          key={role}
                          className={`inline-flex items-center px-2 py-1 rounded-full text-[10px] font-bold uppercase tracking-wider ${
                            role === 'admin'
                              ? 'bg-purple-500/10 text-purple-400 border border-purple-500/20'
                              : role === 'developer'
                                ? 'bg-blue-500/10 text-blue-400 border border-blue-500/20'
                                : 'bg-slate-500/10 text-slate-400 border border-slate-500/20'
                          }`}
                        >
                          {role}
                        </span>
                      ))}
                    </div>
                  </td>
                  <td className="p-5 text-text-secondary">
                    {new Date(user.created).toLocaleDateString()}
                  </td>
                  <td className="p-5">-</td>
                  <td className="p-5">
                    <div className="flex items-center gap-2">
                      <span
                        className={`size-2 rounded-full ${user.enabled ? 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.4)]' : 'bg-gray-500'}`}
                      ></span>
                      <span className="text-text-secondary font-medium">
                        {user.enabled ? 'Active' : 'Inactive'}
                      </span>
                    </div>
                  </td>
                  <td className="p-5 text-right">
                    <div className="flex items-center justify-end gap-2">
                      <Link
                        to={`/users/${user.id}`}
                        className="p-2 text-text-secondary hover:text-primary transition-all rounded-lg hover:bg-surface-highlight"
                      >
                        <span className="material-symbols-outlined">edit</span>
                      </Link>
                      <button
                        onClick={() => handleDeleteUser(user.id)}
                        disabled={currentUser?.id === user.id}
                        className="p-2 text-text-secondary hover:text-red-500 transition-all rounded-lg hover:bg-surface-highlight disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:text-text-secondary"
                        title={
                          currentUser?.id === user.id
                            ? 'You cannot delete your own account'
                            : 'Delete user'
                        }
                      >
                        <span className="material-symbols-outlined">delete</span>
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {/* Pagination Controls */}
        <div className="flex flex-col sm:flex-row items-center justify-between p-5 border-t border-border-dark gap-4">
          <div className="text-xs text-text-secondary font-medium">
            Showing{' '}
            <span className="font-bold text-text-foreground">
              {filteredUsers.length > 0 ? startIndex + 1 : 0}
            </span>{' '}
            to{' '}
            <span className="font-bold text-text-foreground">
              {Math.min(startIndex + itemsPerPage, filteredUsers.length)}
            </span>{' '}
            of <span className="font-bold text-text-foreground">{filteredUsers.length}</span>{' '}
            results
          </div>
          {totalPages > 1 && (
            <div className="flex items-center gap-2">
              <button
                onClick={() => handlePageChange(currentPage - 1)}
                disabled={currentPage === 1}
                className="p-2 rounded-lg border border-border-dark text-text-secondary hover:bg-surface-highlight disabled:opacity-30 disabled:cursor-not-allowed transition-all"
              >
                <span className="material-symbols-outlined text-[20px]">chevron_left</span>
              </button>

              {Array.from({ length: totalPages }, (_, i) => i + 1).map((page) => (
                <button
                  key={page}
                  onClick={() => handlePageChange(page)}
                  className={`w-9 h-9 flex items-center justify-center rounded-lg text-sm font-bold transition-all ${
                    currentPage === page
                      ? 'bg-primary text-white shadow-lg shadow-primary/20'
                      : 'border border-border-dark text-text-secondary hover:bg-surface-highlight'
                  }`}
                >
                  {page}
                </button>
              ))}

              <button
                onClick={() => handlePageChange(currentPage + 1)}
                disabled={currentPage === totalPages}
                className="p-2 rounded-lg border border-border-dark text-text-secondary hover:bg-surface-highlight disabled:opacity-30 disabled:cursor-not-allowed transition-all"
              >
                <span className="material-symbols-outlined text-[20px]">chevron_right</span>
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Create User Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="bg-surface-dark border border-border-dark rounded-2xl shadow-xl max-w-md w-full mx-4 max-h-[90vh] overflow-y-auto">
            <div className="p-6 border-b border-border-dark flex items-center justify-between">
              <h3 className="text-xl font-bold text-text-foreground">Create New User</h3>
              <button
                onClick={() => setShowCreateModal(false)}
                className="p-2 text-text-secondary hover:text-text-foreground transition-colors rounded-lg hover:bg-surface-highlight"
              >
                <span className="material-symbols-outlined">close</span>
              </button>
            </div>

            <form onSubmit={handleCreateUser} className="p-6 space-y-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-2">
                  Username <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={newUser.username}
                  onChange={(e) => setNewUser({ ...newUser, username: e.target.value })}
                  className="w-full px-4 py-2 bg-background-dark border border-border-dark rounded-lg text-text-foreground focus:border-primary focus:outline-none"
                  required
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-text-secondary mb-2">
                  Email <span className="text-red-500">*</span>
                </label>
                <input
                  type="email"
                  value={newUser.email}
                  onChange={(e) => setNewUser({ ...newUser, email: e.target.value })}
                  className="w-full px-4 py-2 bg-background-dark border border-border-dark rounded-lg text-text-foreground focus:border-primary focus:outline-none"
                  required
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-text-secondary mb-2">
                  Password <span className="text-red-500">*</span>
                </label>
                <input
                  type="password"
                  value={newUser.password}
                  onChange={(e) => setNewUser({ ...newUser, password: e.target.value })}
                  className="w-full px-4 py-2 bg-background-dark border border-border-dark rounded-lg text-text-foreground focus:border-primary focus:outline-none"
                  required
                  minLength={8}
                />
                <p className="text-xs text-text-secondary mt-1">Minimum 8 characters</p>
              </div>

              <div>
                <label className="block text-sm font-medium text-text-secondary mb-2">Roles</label>
                <div className="space-y-2">
                  {['admin', 'developer'].map((role) => (
                    <label key={role} className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={newUser.roles.includes(role)}
                        onChange={() => toggleRole(role)}
                        className="w-4 h-4 rounded border-border-dark bg-transparent text-primary focus:ring-primary cursor-pointer"
                      />
                      <span className="text-text-foreground capitalize">{role}</span>
                    </label>
                  ))}
                </div>
              </div>

              <div>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={newUser.enabled}
                    onChange={(e) => setNewUser({ ...newUser, enabled: e.target.checked })}
                    className="w-4 h-4 rounded border-border-dark bg-transparent text-primary focus:ring-primary cursor-pointer"
                  />
                  <span className="text-text-foreground">Enabled</span>
                </label>
              </div>

              <div className="flex gap-3 pt-4">
                <button
                  type="button"
                  onClick={() => setShowCreateModal(false)}
                  className="flex-1 px-4 py-2 bg-background-dark border border-border-dark text-text-foreground rounded-lg hover:bg-surface-highlight transition-all"
                  disabled={creating}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={creating}
                  className="flex-1 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary-dark transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                >
                  {creating ? (
                    <>
                      <span className="material-symbols-outlined animate-spin text-[18px]">
                        refresh
                      </span>
                      Creating...
                    </>
                  ) : (
                    <>
                      <span className="material-symbols-outlined text-[18px]">add</span>
                      Create User
                    </>
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};

export default UserManagement;
