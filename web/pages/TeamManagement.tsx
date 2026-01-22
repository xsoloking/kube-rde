import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { teamsApi, Team, CreateTeamRequest } from '../services/api';

const TeamManagement: React.FC = () => {
  const [teams, setTeams] = useState<Team[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [creating, setCreating] = useState(false);

  // Create team form state
  const [newTeam, setNewTeam] = useState<CreateTeamRequest>({
    name: '',
    display_name: '',
  });

  // Fetch teams on mount
  useEffect(() => {
    loadTeams();
  }, []);

  const loadTeams = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await teamsApi.list();
      setTeams(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load teams';
      setError(message);
      console.error('Failed to load teams:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateTeam = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newTeam.name || !newTeam.display_name) {
      alert('Team name and display name are required');
      return;
    }

    try {
      setCreating(true);
      await teamsApi.create(newTeam);
      setShowCreateModal(false);
      setNewTeam({ name: '', display_name: '' });
      await loadTeams();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create team';
      alert('Error: ' + message);
    } finally {
      setCreating(false);
    }
  };

  const handleDeleteTeam = async (teamId: number, teamName: string) => {
    const confirmMessage = `Are you sure you want to delete team "${teamName}"?

This will:
- Remove the team from the system
- Delete the associated Kubernetes namespace
- NOT delete users (they will become unassigned)

Type "DELETE" to confirm:`;

    const confirmation = prompt(confirmMessage);
    if (confirmation !== 'DELETE') {
      return;
    }

    try {
      await teamsApi.delete(teamId);
      await loadTeams();
      alert('Team deleted successfully');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      alert('Failed to delete team: ' + message);
    }
  };

  // Filter teams
  const filteredTeams = teams.filter((team) => {
    const matchesSearch =
      !search ||
      team.name.toLowerCase().includes(search.toLowerCase()) ||
      team.display_name.toLowerCase().includes(search.toLowerCase());

    const matchesStatus =
      !statusFilter ||
      (statusFilter === 'active' && team.status === 'active') ||
      (statusFilter === 'suspended' && team.status === 'suspended');

    return matchesSearch && matchesStatus;
  });

  return (
    <div className="p-8 lg:p-12 max-w-[1400px] mx-auto space-y-8 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-6">
        <div className="flex flex-col gap-2">
          <h2 className="text-4xl font-bold tracking-tight">Team Management</h2>
          <p className="text-text-secondary max-w-2xl text-lg">
            Manage teams, their members, and resource quotas for multi-tenant isolation.
          </p>
        </div>
        <button
          onClick={() => setShowCreateModal(true)}
          className="flex items-center gap-2 bg-primary hover:bg-primary-dark text-white px-6 py-3 rounded-xl font-bold shadow-xl shadow-primary/20 transition-all active:scale-95"
        >
          <span className="material-symbols-outlined text-[22px]">add</span>
          <span>Add Team</span>
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {[
          {
            label: 'Total Teams',
            val: loading ? '-' : teams.length.toString(),
            icon: 'groups',
            col: 'text-primary',
          },
          {
            label: 'Active Teams',
            val: loading ? '-' : teams.filter((t) => t.status === 'active').length.toString(),
            icon: 'verified',
            col: 'text-emerald-500',
          },
          {
            label: 'Suspended Teams',
            val: loading ? '-' : teams.filter((t) => t.status === 'suspended').length.toString(),
            icon: 'pause_circle',
            col: 'text-amber-500',
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

      {/* Filters */}
      <div className="flex flex-col lg:flex-row gap-4 justify-between items-stretch lg:items-center bg-surface-dark p-4 rounded-xl border border-border-dark">
        <div className="flex flex-1 w-full lg:w-auto gap-4 flex-col sm:flex-row">
          <div className="relative flex-1 max-w-md group">
            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-text-secondary group-focus-within:text-primary transition-colors">
              <span className="material-symbols-outlined">search</span>
            </span>
            <input
              className="w-full h-11 pl-10 pr-4 bg-background-dark border border-border-dark rounded-xl focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all placeholder:text-text-secondary/50 text-sm"
              placeholder="Search by team name..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              type="text"
            />
          </div>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="h-11 pl-4 pr-10 bg-background-dark border border-border-dark rounded-xl appearance-none text-sm focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none cursor-pointer min-w-[140px]"
          >
            <option value="">All Status</option>
            <option value="active">Active</option>
            <option value="suspended">Suspended</option>
          </select>
        </div>
      </div>

      {/* Teams Table */}
      <div className="w-full overflow-hidden rounded-2xl border border-border-dark bg-surface-dark shadow-xl">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-background-dark/50 border-b border-border-dark">
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Team
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Namespace
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Status
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                  Created
                </th>
                <th className="p-5 text-[10px] font-bold uppercase tracking-widest text-text-secondary text-right">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border-dark font-body text-sm">
              {loading && (
                <tr>
                  <td colSpan={5} className="p-8 text-center text-text-secondary">
                    <div className="flex items-center justify-center gap-2">
                      <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary"></div>
                      <span>Loading teams...</span>
                    </div>
                  </td>
                </tr>
              )}
              {error && (
                <tr>
                  <td colSpan={5} className="p-8 text-center text-red-500">
                    <span className="material-symbols-outlined text-[24px] block mb-2">
                      error_outline
                    </span>
                    {error}
                  </td>
                </tr>
              )}
              {!loading && !error && filteredTeams.length === 0 && (
                <tr>
                  <td colSpan={5} className="p-8 text-center text-text-secondary">
                    <span className="material-symbols-outlined text-[48px] block mb-2 opacity-30">
                      groups
                    </span>
                    <p className="text-lg mb-1">No teams found</p>
                    <p className="text-sm opacity-70">Create a team to get started</p>
                  </td>
                </tr>
              )}
              {!loading &&
                !error &&
                filteredTeams.map((team) => (
                  <tr key={team.id} className="hover:bg-surface-highlight/50 transition-colors">
                    <td className="p-5">
                      <div className="flex items-center gap-4">
                        <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center">
                          <span className="material-symbols-outlined text-primary text-[20px]">
                            groups
                          </span>
                        </div>
                        <div>
                          <p className="font-bold text-white">{team.display_name}</p>
                          <p className="text-text-secondary text-xs">{team.name}</p>
                        </div>
                      </div>
                    </td>
                    <td className="p-5">
                      <code className="px-2 py-1 bg-background-dark rounded text-xs text-primary">
                        {team.namespace}
                      </code>
                    </td>
                    <td className="p-5">
                      <span
                        className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-bold ${
                          team.status === 'active'
                            ? 'bg-emerald-500/10 text-emerald-500'
                            : 'bg-amber-500/10 text-amber-500'
                        }`}
                      >
                        <span
                          className={`w-1.5 h-1.5 rounded-full ${
                            team.status === 'active' ? 'bg-emerald-500' : 'bg-amber-500'
                          }`}
                        ></span>
                        {team.status === 'active' ? 'Active' : 'Suspended'}
                      </span>
                    </td>
                    <td className="p-5 text-text-secondary text-sm">
                      {new Date(team.created_at).toLocaleDateString()}
                    </td>
                    <td className="p-5">
                      <div className="flex justify-end gap-2">
                        <Link
                          to={`/teams/${team.id}/members`}
                          className="p-2 rounded-lg hover:bg-surface-highlight text-text-secondary hover:text-white transition-colors"
                          title="View Members"
                        >
                          <span className="material-symbols-outlined text-[20px]">group</span>
                        </Link>
                        <Link
                          to={`/teams/${team.id}/quota`}
                          className="p-2 rounded-lg hover:bg-surface-highlight text-text-secondary hover:text-white transition-colors"
                          title="Edit Quota"
                        >
                          <span className="material-symbols-outlined text-[20px]">tune</span>
                        </Link>
                        <Link
                          to={`/teams/${team.id}/edit`}
                          className="p-2 rounded-lg hover:bg-surface-highlight text-text-secondary hover:text-white transition-colors"
                          title="Edit Team"
                        >
                          <span className="material-symbols-outlined text-[20px]">edit</span>
                        </Link>
                        <button
                          onClick={() => handleDeleteTeam(team.id, team.display_name)}
                          className="p-2 rounded-lg hover:bg-red-500/10 text-text-secondary hover:text-red-500 transition-colors"
                          title="Delete Team"
                        >
                          <span className="material-symbols-outlined text-[20px]">delete</span>
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Create Team Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="bg-surface-dark rounded-2xl p-8 w-full max-w-md border border-border-dark shadow-2xl">
            <div className="flex justify-between items-center mb-6">
              <h3 className="text-2xl font-bold">Create Team</h3>
              <button
                onClick={() => setShowCreateModal(false)}
                className="p-2 hover:bg-surface-highlight rounded-lg transition-colors"
              >
                <span className="material-symbols-outlined">close</span>
              </button>
            </div>

            <form onSubmit={handleCreateTeam} className="space-y-4">
              <div>
                <label className="block text-sm font-bold text-text-secondary mb-2">
                  Team Name (ID)
                </label>
                <input
                  type="text"
                  value={newTeam.name}
                  onChange={(e) =>
                    setNewTeam({
                      ...newTeam,
                      name: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'),
                    })
                  }
                  placeholder="e.g., ai-team"
                  className="w-full h-11 px-4 bg-background-dark border border-border-dark rounded-xl focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all text-sm"
                  required
                />
                <p className="text-xs text-text-secondary mt-1">
                  Lowercase letters, numbers, and hyphens only. Used for namespace.
                </p>
              </div>

              <div>
                <label className="block text-sm font-bold text-text-secondary mb-2">
                  Display Name
                </label>
                <input
                  type="text"
                  value={newTeam.display_name}
                  onChange={(e) => setNewTeam({ ...newTeam, display_name: e.target.value })}
                  placeholder="e.g., AI Research Team"
                  className="w-full h-11 px-4 bg-background-dark border border-border-dark rounded-xl focus:ring-2 focus:ring-primary/50 focus:border-primary outline-none transition-all text-sm"
                  required
                />
              </div>

              <div className="flex gap-3 pt-4">
                <button
                  type="button"
                  onClick={() => setShowCreateModal(false)}
                  className="flex-1 h-11 bg-background-dark border border-border-dark rounded-xl font-bold hover:bg-surface-highlight transition-all"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={creating}
                  className="flex-1 h-11 bg-primary hover:bg-primary-dark text-white rounded-xl font-bold transition-all disabled:opacity-50"
                >
                  {creating ? 'Creating...' : 'Create Team'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};

export default TeamManagement;
