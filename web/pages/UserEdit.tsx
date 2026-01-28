import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  usersApi,
  sshKeysApi,
  teamsApi,
  User as ApiUser,
  SSHKey,
  Team,
  TeamQuotaItem,
} from '../services/api';
import { useAuth } from '../contexts/AuthContext';

const UserEdit: React.FC = () => {
  const { id } = useParams();
  const { user: currentUser } = useAuth();
  const [user, setUser] = useState<ApiUser | null>(null);
  const [sshKeys, setSSHKeys] = useState<SSHKey[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [selectedTeamId, setSelectedTeamId] = useState<number | null>(null);
  const [teamQuotas, setTeamQuotas] = useState<TeamQuotaItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showAddKeyModal, setShowAddKeyModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [newKeyPublic, setNewKeyPublic] = useState('');

  // Check permissions
  const isAdmin = currentUser?.roles?.includes('admin') || false;
  const isOwnProfile = currentUser?.id === id;
  const canEditSSHKeys = isOwnProfile || isAdmin;

  useEffect(() => {
    if (id) {
      loadUserData();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  const loadUserData = async () => {
    if (!id) return;

    try {
      setLoading(true);

      // First, fetch user data to check permissions
      const userData = await usersApi.get(id);
      setUser(userData as ApiUser);

      // Check if current user has permission to view this profile
      const viewingOwnProfile = currentUser?.id === id;
      const isAdminUser = currentUser?.roles?.includes('admin') || false;

      if (!viewingOwnProfile && !isAdminUser) {
        // Developer trying to access another user's profile
        setError('You do not have permission to view this user profile.');
        setLoading(false);
        return;
      }

      // Load remaining data
      const dataPromises: Promise<unknown>[] = [sshKeysApi.list(id)];

      if (isAdminUser) {
        dataPromises.push(teamsApi.list());
      }

      const results = await Promise.all(dataPromises);
      const keysData = results[0] as SSHKey[];
      const teamsData = isAdminUser ? (results[1] as Team[]) : [];

      setSSHKeys(keysData || []);
      setTeams(teamsData || []);

      // Set selected team from user data
      const userTeamId = (userData as ApiUser).team_id || null;
      setSelectedTeamId(userTeamId);

      // Load team quotas if user belongs to a team
      if (userTeamId) {
        try {
          const teamQuotaData = await teamsApi.getQuota(userTeamId);
          setTeamQuotas(teamQuotaData || []);
        } catch (err) {
          console.error('Failed to load team quotas:', err);
          setTeamQuotas([]);
        }
      }
    } catch (err) {
      setError('Failed to load user data');
      console.error('Failed to load user data:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleTeamChange = async (teamId: number | null) => {
    if (!id || !isAdmin) return;

    try {
      setSaving(true);
      await usersApi.update(id, { team_id: teamId });
      setSelectedTeamId(teamId);
      // Also update user state
      if (user) {
        setUser({ ...user, team_id: teamId || undefined });
      }

      // Load team quotas if new team selected
      if (teamId) {
        try {
          const teamQuotaData = await teamsApi.getQuota(teamId);
          setTeamQuotas(teamQuotaData || []);
        } catch (err) {
          console.error('Failed to load team quotas:', err);
          setTeamQuotas([]);
        }
      } else {
        setTeamQuotas([]);
      }
    } catch (err) {
      console.error('Failed to update team:', err);
      alert('Failed to update team assignment');
    } finally {
      setSaving(false);
    }
  };

  const handleAddSSHKey = async () => {
    if (!id || !newKeyName || !newKeyPublic) {
      alert('Please provide both key name and public key');
      return;
    }

    try {
      const newKey = await sshKeysApi.create(id, {
        name: newKeyName,
        public_key: newKeyPublic,
      });
      setSSHKeys([...sshKeys, newKey]);
      setShowAddKeyModal(false);
      setNewKeyName('');
      setNewKeyPublic('');
      alert('SSH key added successfully');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to add SSH key: ' + message);
    }
  };

  const handleDeleteSSHKey = async (keyId: string) => {
    if (!id) return;
    if (!confirm('Are you sure you want to delete this SSH key?')) return;

    try {
      await sshKeysApi.delete(id, keyId);
      setSSHKeys(sshKeys.filter((k) => k.id !== keyId));
      alert('SSH key deleted successfully');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to delete SSH key: ' + message);
    }
  };

  const handleDeleteUser = async () => {
    if (!id || !user) return;

    // Prevent self-deletion
    if (isOwnProfile) {
      alert('You cannot delete your own account');
      return;
    }

    // Show confirmation dialog with cascade deletion warning
    const confirmMessage = `⚠️ WARNING: This will permanently delete user "${user.username}" and ALL associated data including:

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
      setSaving(true);
      await usersApi.delete(id);
      alert('User and all associated data deleted successfully');
      // Redirect to user management page
      window.location.href = '/#/users';
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to delete user: ' + message);
      setSaving(false);
    }
  };

  // Show loading state
  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mx-auto mb-4"></div>
          <p className="text-text-secondary">Loading user data...</p>
        </div>
      </div>
    );
  }

  // Show error state (including permission denied)
  if (error) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center max-w-md">
          <div className="mb-6">
            <span className="material-symbols-outlined text-6xl text-red-500 mb-4 block">
              block
            </span>
          </div>
          <h1 className="text-3xl font-bold text-text-foreground mb-4">Access Denied</h1>
          <p className="text-text-secondary mb-6">{error}</p>
          <a
            href="/#/"
            className="inline-flex items-center gap-2 px-6 py-3 bg-primary text-white rounded-lg text-sm font-bold uppercase tracking-widest hover:bg-primary-dark transition-all"
          >
            <span className="material-symbols-outlined">arrow_back</span>
            Back to Dashboard
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col h-full animate-fade-in overflow-hidden">
      <div className="flex-1 overflow-y-auto px-8 py-8 custom-scrollbar">
        <div className="max-w-6xl mx-auto w-full flex flex-col gap-8 pb-24">
          <div className="flex flex-col md:flex-row md:items-end justify-between gap-4">
            <div>
              <h1 className="text-3xl md:text-4xl font-bold text-text-foreground tracking-tight mb-2">
                {isOwnProfile
                  ? 'My Profile'
                  : `Edit User: ${loading ? '...' : user?.username || 'Unknown'}`}
              </h1>
              <p className="text-text-secondary text-base max-w-2xl">
                {isOwnProfile
                  ? 'View your profile, SSH keys, and resource quotas.'
                  : 'Manage user profile, access roles, SSH keys, and resource quotas.'}
              </p>
            </div>
            <div className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-emerald-500/10 border border-emerald-500/20">
              <span className="size-2 rounded-full bg-emerald-500 animate-pulse"></span>
              <span className="text-emerald-400 text-sm font-bold uppercase tracking-widest">
                Active Session
              </span>
            </div>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-12 gap-8">
            <div className="xl:col-span-4 flex flex-col gap-6">
              <section className="bg-surface-dark rounded-2xl border border-border-dark overflow-hidden shadow-xl">
                <div className="px-6 py-4 border-b border-border-dark flex justify-between items-center bg-background-dark/30">
                  <h2 className="text-sm font-bold text-text-foreground uppercase tracking-widest">
                    Identity
                  </h2>
                  <button className="text-text-secondary hover:text-text-foreground transition-colors">
                    <span className="material-symbols-outlined text-[20px]">more_horiz</span>
                  </button>
                </div>
                <div className="p-6 flex flex-col gap-6">
                  <div className="flex items-center gap-4">
                    <div className="relative group cursor-pointer shrink-0">
                      <div className="size-20 rounded-full bg-gradient-to-br from-primary to-primary-dark flex items-center justify-center border-2 border-border-dark group-hover:border-primary transition-all duration-300">
                        <span className="material-symbols-outlined text-text-foreground text-4xl">
                          person
                        </span>
                      </div>
                    </div>
                    <div>
                      <p className="text-text-foreground font-bold text-xl">
                        {user?.full_name || user?.name || user?.username || 'Unknown User'}
                      </p>
                      <p className="text-text-secondary text-xs font-medium mt-1">
                        Created{' '}
                        {user?.created_at
                          ? new Date(user.created_at).toLocaleDateString('en-US', {
                              year: 'numeric',
                              month: 'short',
                              day: 'numeric',
                            })
                          : 'Unknown'}
                      </p>
                    </div>
                  </div>
                  <div className="space-y-5">
                    <div>
                      <label className="block text-text-secondary text-[10px] uppercase tracking-widest font-bold mb-2">
                        Username
                      </label>
                      <input
                        className="w-full bg-background-dark border border-border-dark rounded-xl px-4 py-3 text-text-foreground opacity-60 cursor-not-allowed text-sm"
                        disabled
                        value={user?.username || ''}
                      />
                    </div>
                    <div>
                      <label className="block text-text-secondary text-[10px] uppercase tracking-widest font-bold mb-2">
                        Email Address
                      </label>
                      <input
                        className="w-full bg-background-dark border border-border-dark rounded-xl px-4 py-3 text-text-foreground opacity-60 cursor-not-allowed text-sm"
                        type="email"
                        disabled
                        value={user?.email || ''}
                      />
                    </div>
                    <div>
                      <label className="block text-text-secondary text-[10px] uppercase tracking-widest font-bold mb-2">
                        Role Assignment
                      </label>
                      <div className="bg-background-dark border border-border-dark rounded-xl p-2.5 flex flex-wrap gap-2 min-h-[50px]">
                        {user?.roles &&
                          user.roles.map((role, index) => (
                            <div
                              key={index}
                              className="flex items-center gap-1.5 bg-primary/20 text-primary border border-primary/20 px-2.5 py-1 rounded-lg text-xs font-bold uppercase tracking-wider"
                            >
                              <span>{role}</span>
                            </div>
                          ))}
                        {(!user?.roles || user.roles.length === 0) && (
                          <p className="text-text-secondary text-xs py-2">No roles assigned</p>
                        )}
                      </div>
                    </div>
                    {isAdmin && (
                      <div>
                        <label className="block text-text-secondary text-[10px] uppercase tracking-widest font-bold mb-2">
                          Team Assignment
                        </label>
                        <select
                          value={selectedTeamId || ''}
                          onChange={(e) =>
                            handleTeamChange(e.target.value ? parseInt(e.target.value, 10) : null)
                          }
                          disabled={saving}
                          className="w-full bg-background-dark border border-border-dark rounded-xl px-4 py-3 text-text-foreground text-sm focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all appearance-none cursor-pointer disabled:opacity-50"
                        >
                          <option value="">No Team</option>
                          {teams.map((team) => (
                            <option key={team.id} value={team.id}>
                              {team.display_name}
                            </option>
                          ))}
                        </select>
                        <p className="text-text-secondary text-[10px] mt-1">
                          {selectedTeamId
                            ? `Namespace: ${teams.find((t) => t.id === selectedTeamId)?.namespace || 'N/A'}`
                            : 'User resources will use default namespace'}
                        </p>
                      </div>
                    )}
                  </div>
                  <button className="w-full flex items-center justify-between px-4 py-3.5 bg-background-dark hover:bg-surface-highlight border border-border-dark rounded-xl text-xs font-bold uppercase tracking-widest text-text-secondary hover:text-text-foreground transition-all group">
                    <span className="flex items-center gap-2">
                      <span className="material-symbols-outlined text-[20px] group-hover:text-primary transition-colors">
                        lock_reset
                      </span>
                      Password Settings
                    </span>
                    <span className="material-symbols-outlined text-[20px] group-hover:rotate-180 transition-transform">
                      expand_more
                    </span>
                  </button>
                </div>
              </section>

              <section className="bg-surface-dark rounded-2xl border border-border-dark overflow-hidden flex-1 shadow-xl">
                <div className="px-6 py-4 border-b border-border-dark flex justify-between items-center bg-background-dark/30">
                  <h2 className="text-sm font-bold text-text-foreground uppercase tracking-widest">
                    SSH Keys
                  </h2>
                  {canEditSSHKeys && (
                    <button
                      onClick={() => setShowAddKeyModal(true)}
                      className="flex items-center gap-1 text-primary text-xs font-bold uppercase tracking-widest hover:text-primary/80 transition-colors"
                    >
                      <span className="material-symbols-outlined text-[18px]">add</span> Add Key
                    </button>
                  )}
                </div>
                <div className="divide-y divide-border-dark">
                  {sshKeys.length === 0 ? (
                    <div className="p-8 text-center text-text-secondary">
                      <span className="material-symbols-outlined text-4xl mb-2 block">key_off</span>
                      <p className="text-sm">No SSH keys configured</p>
                      <p className="text-xs mt-1">Click "Add Key" to add your first SSH key</p>
                    </div>
                  ) : (
                    sshKeys.map((key) => (
                      <div
                        key={key.id}
                        className="p-5 flex items-start gap-3 hover:bg-surface-highlight transition-colors"
                      >
                        <div className="mt-1 text-text-secondary shrink-0">
                          <span className="material-symbols-outlined">key</span>
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex justify-between items-start">
                            <p className="text-text-foreground text-sm font-bold truncate">
                              {key.name}
                            </p>
                            {canEditSSHKeys && (
                              <button
                                onClick={() => handleDeleteSSHKey(key.id)}
                                className="text-red-400 hover:text-red-300 transition-colors"
                                title="Delete Key"
                              >
                                <span className="material-symbols-outlined text-[18px]">
                                  delete
                                </span>
                              </button>
                            )}
                          </div>
                          <p className="text-text-secondary text-[10px] font-mono mt-1 truncate">
                            {key.public_key.substring(0, 50)}...
                          </p>
                          <p className="text-slate-600 text-[10px] font-bold uppercase tracking-wider mt-2">
                            Added{' '}
                            {new Date(key.added_at).toLocaleDateString('en-US', {
                              month: 'short',
                              day: 'numeric',
                              year: 'numeric',
                            })}{' '}
                            • {key.fingerprint}
                          </p>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </section>
            </div>

            <div className="xl:col-span-8 flex flex-col gap-6">
              <section className="bg-surface-dark rounded-2xl border border-border-dark overflow-hidden h-full shadow-xl">
                <div className="px-6 py-5 border-b border-border-dark flex flex-wrap justify-between items-center gap-4 bg-background-dark/30">
                  <div>
                    <h2 className="text-xl font-bold text-text-foreground">Team Resource Quotas</h2>
                    <p className="text-text-secondary text-sm mt-1">
                      {selectedTeamId ? (
                        <>
                          Resource quotas are managed at team level.{' '}
                          {isAdmin && (
                            <Link
                              to={`/teams/${selectedTeamId}/quota`}
                              className="text-primary hover:underline"
                            >
                              Edit team quotas →
                            </Link>
                          )}
                        </>
                      ) : (
                        'You are not assigned to any team. Contact an administrator to join a team.'
                      )}
                    </p>
                  </div>
                  {selectedTeamId && (
                    <div className="flex items-center gap-2 px-3 py-1.5 bg-primary/10 border border-primary/20 rounded-lg">
                      <span className="material-symbols-outlined text-primary text-[18px]">
                        groups
                      </span>
                      <span className="text-primary text-[10px] font-bold uppercase tracking-widest">
                        Team Managed
                      </span>
                    </div>
                  )}
                </div>
                <div className="p-8 space-y-8">
                  {loading ? (
                    <div className="text-center py-8">
                      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mx-auto"></div>
                      <p className="text-text-secondary mt-4">Loading quotas...</p>
                    </div>
                  ) : selectedTeamId && teamQuotas.length > 0 ? (
                    /* Team Quotas Display (Read-Only) */
                    <>
                      {/* Compute Resources */}
                      {teamQuotas.filter(
                        (q) => q.resource_type === 'cpu' || q.resource_type === 'memory',
                      ).length > 0 && (
                        <div>
                          <div className="flex items-center gap-2 mb-4">
                            <span className="material-symbols-outlined text-text-secondary text-[20px]">
                              memory
                            </span>
                            <label className="text-text-foreground font-bold text-base">
                              Compute Resources
                            </label>
                          </div>
                          <div className="grid grid-cols-2 gap-4">
                            {teamQuotas
                              .filter(
                                (q) => q.resource_type === 'cpu' || q.resource_type === 'memory',
                              )
                              .map((q) => (
                                <div
                                  key={q.resource_config_id}
                                  className="bg-background-dark border border-border-dark rounded-lg p-4"
                                >
                                  <p className="text-text-secondary text-xs font-bold uppercase tracking-wider mb-1">
                                    {q.display_name}
                                  </p>
                                  <p className="text-2xl font-bold">
                                    {q.quota}{' '}
                                    <span className="text-sm text-text-secondary font-normal">
                                      {q.unit}
                                    </span>
                                  </p>
                                </div>
                              ))}
                          </div>
                        </div>
                      )}

                      {/* Storage */}
                      {teamQuotas.filter((q) => q.resource_type === 'storage').length > 0 && (
                        <div>
                          <div className="flex items-center gap-2 mb-4">
                            <span className="material-symbols-outlined text-text-secondary text-[20px]">
                              hard_drive
                            </span>
                            <label className="text-text-foreground font-bold text-base">
                              Storage
                            </label>
                          </div>
                          <div className="bg-background-dark border border-border-dark rounded-lg p-4">
                            <div className="space-y-3">
                              {teamQuotas
                                .filter((q) => q.resource_type === 'storage')
                                .map((q) => (
                                  <div
                                    key={q.resource_config_id}
                                    className="flex items-center justify-between"
                                  >
                                    <span className="text-text-secondary text-sm">
                                      {q.display_name}
                                    </span>
                                    <span className="text-text-foreground font-bold">
                                      {q.quota} {q.unit}
                                    </span>
                                  </div>
                                ))}
                            </div>
                          </div>
                        </div>
                      )}

                      {/* GPU */}
                      {teamQuotas.filter((q) => q.resource_type === 'gpu').length > 0 && (
                        <div>
                          <div className="flex items-center gap-2 mb-4">
                            <span className="material-symbols-outlined text-text-secondary text-[20px]">
                              videogame_asset
                            </span>
                            <label className="text-text-foreground font-bold text-base">
                              GPU Resources
                            </label>
                          </div>
                          <div className="bg-background-dark border border-border-dark rounded-lg p-4">
                            <div className="space-y-3">
                              {teamQuotas
                                .filter((q) => q.resource_type === 'gpu')
                                .map((q) => (
                                  <div
                                    key={q.resource_config_id}
                                    className="flex items-center justify-between"
                                  >
                                    <div>
                                      <span className="text-text-foreground text-sm font-medium">
                                        {q.display_name}
                                      </span>
                                      <p className="text-text-secondary text-xs">
                                        {q.resource_name}
                                      </p>
                                    </div>
                                    <span className="text-text-foreground font-bold">
                                      {q.quota} {q.unit}
                                    </span>
                                  </div>
                                ))}
                            </div>
                          </div>
                        </div>
                      )}
                    </>
                  ) : selectedTeamId ? (
                    <div className="text-center py-8 text-text-secondary">
                      <span className="material-symbols-outlined text-[48px] block mb-2 opacity-30">
                        inventory_2
                      </span>
                      <p>No team quotas configured</p>
                      {isAdmin && (
                        <Link
                          to={`/teams/${selectedTeamId}/quota`}
                          className="inline-flex items-center gap-2 mt-4 px-4 py-2 bg-primary/10 text-primary rounded-lg hover:bg-primary/20 transition-colors"
                        >
                          <span className="material-symbols-outlined text-[18px]">settings</span>
                          Configure Team Quotas
                        </Link>
                      )}
                    </div>
                  ) : (
                    /* No team assigned */
                    <div className="text-center py-12 text-text-secondary">
                      <span className="material-symbols-outlined text-[64px] block mb-4 opacity-30">
                        group_off
                      </span>
                      <p className="text-lg font-medium mb-2">No Team Assigned</p>
                      <p className="text-sm max-w-md mx-auto">
                        You are not assigned to any team. Resource quotas are managed at the team
                        level. Please contact an administrator to be assigned to a team.
                      </p>
                    </div>
                  )}
                </div>
              </section>
            </div>
          </div>
        </div>
      </div>

      {isAdmin && (
        <div className="shrink-0 bg-background-dark border-t border-border-dark p-6 z-20">
          <div className="max-w-6xl mx-auto w-full flex flex-col sm:flex-row items-center justify-between gap-4">
            <button
              onClick={handleDeleteUser}
              disabled={isOwnProfile || saving}
              className="text-red-500 hover:text-red-400 text-xs font-bold uppercase tracking-widest flex items-center gap-2 px-5 py-2.5 rounded-xl hover:bg-red-500/10 transition-all active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed"
              title={
                isOwnProfile ? 'You cannot delete your own account' : 'Delete this user account'
              }
            >
              <span className="material-symbols-outlined text-[20px]">delete_forever</span>
              Delete User Account
            </button>
            {selectedTeamId && (
              <p className="text-text-secondary text-sm">
                Quotas managed by team.{' '}
                <Link
                  to={`/teams/${selectedTeamId}/quota`}
                  className="text-primary hover:underline"
                >
                  Edit team quotas →
                </Link>
              </p>
            )}
          </div>
        </div>
      )}

      {/* Add SSH Key Modal */}
      {showAddKeyModal && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
          <div className="bg-surface-dark border border-border-dark rounded-2xl max-w-2xl w-full shadow-2xl">
            <div className="px-6 py-5 border-b border-border-dark flex justify-between items-center">
              <h2 className="text-xl font-bold text-text-foreground">Add SSH Public Key</h2>
              <button
                onClick={() => {
                  setShowAddKeyModal(false);
                  setNewKeyName('');
                  setNewKeyPublic('');
                }}
                className="text-text-secondary hover:text-text-foreground transition-colors"
              >
                <span className="material-symbols-outlined">close</span>
              </button>
            </div>
            <div className="p-6 space-y-5">
              <div>
                <label className="block text-text-secondary text-[10px] uppercase tracking-widest font-bold mb-2">
                  Key Name
                </label>
                <input
                  type="text"
                  value={newKeyName}
                  onChange={(e) => setNewKeyName(e.target.value)}
                  placeholder="e.g., laptop, work-desktop"
                  className="w-full bg-background-dark border border-border-dark rounded-xl px-4 py-3 text-text-foreground focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary transition-all text-sm"
                />
              </div>
              <div>
                <label className="block text-text-secondary text-[10px] uppercase tracking-widest font-bold mb-2">
                  Public Key
                </label>
                <textarea
                  value={newKeyPublic}
                  onChange={(e) => setNewKeyPublic(e.target.value)}
                  placeholder="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC..."
                  rows={6}
                  className="w-full bg-background-dark border border-border-dark rounded-xl px-4 py-3 text-text-foreground font-mono text-xs focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary transition-all resize-none"
                />
                <p className="text-text-secondary text-xs mt-2">
                  Paste your public key here. Usually found in ~/.ssh/id_rsa.pub or
                  ~/.ssh/id_ed25519.pub
                </p>
              </div>
            </div>
            <div className="px-6 py-4 border-t border-border-dark flex items-center justify-end gap-3">
              <button
                onClick={() => {
                  setShowAddKeyModal(false);
                  setNewKeyName('');
                  setNewKeyPublic('');
                }}
                className="h-11 px-8 rounded-xl bg-surface-dark border border-border-dark text-text-foreground font-bold text-sm hover:bg-surface-highlight transition-all active:scale-95"
              >
                Cancel
              </button>
              <button
                onClick={handleAddSSHKey}
                className="h-11 px-8 rounded-xl bg-primary text-white font-bold text-sm hover:bg-primary-dark shadow-xl shadow-primary/20 transition-all flex items-center justify-center gap-2 active:scale-95"
              >
                <span className="material-symbols-outlined text-[18px]">add</span>
                Add Key
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default UserEdit;
