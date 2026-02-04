const API_BASE_URL = ''; // Use relative path (vite proxy in dev, nginx in production)

interface ApiError {
  message: string;
  status: number;
}

class ApiClient {
  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const url = `${API_BASE_URL}${endpoint}`;

    const config: RequestInit = {
      ...options,
      credentials: 'include', // Important: include cookies
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    };

    try {
      const response = await fetch(url, config);

      // Handle 401 Unauthorized
      if (response.status === 401) {
        // Don't redirect if already on login page to avoid redirect loops
        const currentPath = window.location.hash || window.location.pathname;
        if (!currentPath.includes('/login')) {
          // Redirect to login page
          window.location.href =
            '/auth/login?return_url=' + encodeURIComponent(window.location.href);
        }
        throw new Error('Unauthorized');
      }

      if (!response.ok) {
        let errorMessage = `HTTP ${response.status}`;
        const contentType = response.headers.get('content-type');

        if (contentType && contentType.includes('application/json')) {
          try {
            const errorJson = await response.json();
            errorMessage = errorJson.message || errorJson.error || errorMessage;
          } catch {
            // Failed to parse JSON, try text
            const errorText = await response.text();
            errorMessage = errorText || errorMessage;
          }
        } else {
          const errorText = await response.text();
          errorMessage = errorText || errorMessage;
        }

        const error: ApiError = {
          message: errorMessage,
          status: response.status,
        };
        throw error;
      }

      // Handle 204 No Content (empty response body)
      if (response.status === 204) {
        return undefined as T;
      }

      // Check if response has content before parsing JSON
      const contentType = response.headers.get('content-type');
      if (contentType && contentType.includes('application/json')) {
        return await response.json();
      }

      // For non-JSON responses, return empty object
      return {} as T;
    } catch (error) {
      console.error('API request failed:', error);
      throw error;
    }
  }

  async get<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'GET' });
  }

  async post<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'POST',
      body: JSON.stringify(data || {}),
    });
  }

  async put<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'PUT',
      body: JSON.stringify(data || {}),
    });
  }

  async delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'DELETE' });
  }
}

export const api = new ApiClient();

// System Config API (public, no auth required)
export const systemConfigApi = {
  get: () => api.get<SystemConfig>('/api/system-config'),
};

// Auth API
export const authApi = {
  getCurrentUser: () => api.get<User>('/api/me'),
  logout: () => api.post<{ message: string; redirect_to?: string }>('/auth/logout'),
  refresh: () => api.post('/auth/refresh'),
};

// Users API
export const usersApi = {
  list: (first?: number, max?: number) => {
    const params = new URLSearchParams();
    if (first !== undefined) params.append('first', first.toString());
    if (max !== undefined) params.append('max', max.toString());
    return api.get<User[]>(`/api/users?${params.toString()}`);
  },
  get: (id: string) => api.get<User>(`/api/users/${id}`),
  create: (user: CreateUserRequest) =>
    api.post<{ id: string; message: string }>('/api/users', user),
  update: (id: string, user: UpdateUserRequest) => api.put(`/api/users/${id}`, user),
  delete: (id: string) => api.delete(`/api/users/${id}`),
};

// Agents API
export const agentsApi = {
  list: () => api.get<Agent[]>('/api/agents'),
  get: (id: string) => api.get<Agent>(`/api/agents/${id}`),
  create: (agent: CreateAgentRequest) => api.post('/api/agents', agent),
  delete: (id: string) => api.delete(`/api/agents/${id}`),
  scaleUp: (id: string) => api.put(`/api/agents/${id}/scale-up`),
  stop: (id: string) => api.put(`/api/agents/${id}/stop`),
  logs: (id: string) => api.get<{ logs: string }>(`/api/agents/${id}/logs`),
};

// Stats API
export const statsApi = {
  global: () => api.get('/api/stats'),
  update: (config: ResourceConfig) => api.put('/api/config', config),
  traffic: () => api.get('/api/traffic'),
  events: () => api.get('/api/events'),
};

// Workspaces API (Phase 3)
export const workspacesApi = {
  list: () => api.get<{ workspaces: Workspace[] }>('/api/workspaces').then((res) => res.workspaces),
  get: (id: string) => api.get<Workspace>(`/api/workspaces/${id}`),
  create: (workspace: CreateWorkspaceRequest) => api.post<Workspace>('/api/workspaces', workspace),
  update: (id: string, workspace: UpdateWorkspaceRequest) =>
    api.put<Workspace>(`/api/workspaces/${id}`, workspace),
  delete: (id: string) => api.delete(`/api/workspaces/${id}`),
};

// Services API (Phase 3)
export const servicesApi = {
  listByWorkspace: (workspaceId: string) =>
    api
      .get<{ services: Service[] }>(`/api/workspaces/${workspaceId}/services`)
      .then((res) => res.services),
  get: (id: string) => api.get<Service>(`/api/services/${id}`),
  create: (workspaceId: string, service: CreateServiceRequest) =>
    api.post<Service>(`/api/workspaces/${workspaceId}/services`, service),
  update: (id: string, service: UpdateServiceRequest) =>
    api.put<Service>(`/api/services/${id}`, service),
  delete: (id: string) => api.delete(`/api/services/${id}`),
  restart: (id: string) =>
    api.put<{ message: string; deleted_pods: string[] }>(`/api/services/${id}/restart`, {}),
  stop: (id: string) => api.put<{ message: string }>(`/api/services/${id}/stop`, {}),
  start: (id: string) => api.put<{ message: string }>(`/api/services/${id}/start`, {}),
  getLogs: (id: string, container: 'kuberde-agent' | 'workload' = 'workload') =>
    api.get<{ container: string; pod_name: string; logs: string }>(
      `/api/services/${id}/logs?container=${container}`,
    ),
};

// Agent Templates API (Phase 5)
export const agentTemplatesApi = {
  list: () => api.get<AgentTemplate[]>('/api/agent-templates'),
  get: (id: string) => api.get<AgentTemplate>(`/api/agent-templates/${id}`),
  create: (template: Omit<AgentTemplate, 'id' | 'created_at' | 'updated_at'>) =>
    api.post<{ id: string; message: string }>('/api/agent-templates', template),
  update: (id: string, template: Partial<AgentTemplate>) =>
    api.put<AgentTemplate>(`/api/agent-templates/${id}`, template),
  delete: (id: string) => api.delete(`/api/agent-templates/${id}`),
};

// ...

export interface CreateAgentTemplateRequest {
  name: string;
  agent_type: string;
  description?: string;
  docker_image: string;
  default_local_target: string;
  default_external_port: number;
  startup_args?: string;
  env_vars?: Record<string, unknown>;
  security_context?: Record<string, unknown>;
  volume_mounts?: unknown[];
}

// Resource Config API (admin only)
export const resourceConfigApi = {
  get: () => api.get<ResourceConfig>('/api/admin/resource-config'),
  update: (config: ResourceConfig) => api.put<ResourceConfig>('/api/admin/resource-config', config),
};

// User Quota API
export const userQuotaApi = {
  get: (userId: string) => api.get<UserQuota>(`/api/users/${userId}/quota`),
  update: (userId: string, quota: UserQuota) => api.put(`/api/users/${userId}/quota`, quota),
};

// SSH Keys API
export const sshKeysApi = {
  list: (userId: string) => api.get<SSHKey[]>(`/api/users/${userId}/ssh-keys`),
  create: (userId: string, key: CreateSSHKeyRequest) =>
    api.post<SSHKey>(`/api/users/${userId}/ssh-keys`, key),
  delete: (userId: string, keyId: string) => api.delete(`/api/users/${userId}/ssh-keys/${keyId}`),
};

// Types
export interface SystemConfig {
  public_url: string;
  agent_domain: string;
  keycloak_url: string;
  realm_name: string;
}

export interface User {
  id: string;
  username: string;
  email: string;
  full_name?: string;
  name?: string;
  roles: string[];
  enabled: boolean;
  created: number;
  created_at?: string;
  ssh_keys?: SSHKey[];
  team_id?: number;
  team?: Team;
}

export interface ResourceConfig {
  id: number;
  default_cpu_cores: number;
  default_memory_gi: number;
  storage_classes: StorageClassConfig[] | string;
  gpu_types: GPUTypeConfig[] | string;
  created_at: string;
  updated_at: string;
}

export interface StorageClassConfig {
  name: string;
  limit_gi: number;
  is_default?: boolean;
}

export interface GPUTypeConfig {
  model_name: string; // Display name: NVIDIA A100, BIREN 106C
  resource_name: string; // Kubernetes resource: nvidia.com/gpu, birentech.com/gpu
  node_label_key: string; // Node selector key: nvidia.com/model, birentech.com/model
  node_label_value: string; // Node selector value: A100, 106C
  limit: number;
  is_default?: boolean;
}

export interface UserStorageQuotaItem {
  name: string;
  limit_gi: number;
}

export interface UserGPUQuotaItem {
  name: string;
  model_name: string;
  limit: number;
}

export interface UserQuota {
  user_id: string;
  cpu_cores: number;
  memory_gi: number;
  storage_quota: UserStorageQuotaItem[];
  gpu_quota: UserGPUQuotaItem[];
  created_at: string;
  updated_at: string;
}

export interface UpdateUserQuotaRequest {
  cpu_cores?: number;
  memory_gi?: number;
  storage_quota?: UserStorageQuotaItem[];
  gpu_quota?: UserGPUQuotaItem[];
}

export interface CreateUserRequest {
  username: string;
  email: string;
  password: string;
  roles?: string[];
  enabled?: boolean;
}

export interface UpdateUserRequest {
  email?: string;
  enabled?: boolean;
  roles?: string[];
  team_id?: number | null;
}

export interface Agent {
  id: string;
  name: string;
  owner: string;
  status: string;
  created: string;
}

export interface CreateAgentRequest {
  name: string;
  image?: string;
  localTarget?: string;
}

// Workspace Types (Phase 3)
export interface Workspace {
  id: string;
  name: string;
  description?: string;
  owner_id: string;
  owner?: User;
  team_id?: number;
  team?: Team;
  storage_size: string;
  storage_class: string;
  pvc_name?: string;
  created_at: string;
  updated_at: string;
  services?: Service[];
}

export interface CreateWorkspaceRequest {
  name: string;
  description?: string;
  storage_size: string;
  storage_class: string;
}

export interface UpdateWorkspaceRequest {
  name?: string;
  description?: string;
}

// Service Types (Phase 3)
export interface Service {
  id: string;
  name: string;
  local_target: string;
  external_port: number;
  workspace_id: string;
  agent_id?: string;
  remote_proxy?: string; // Agent domain for web access (e.g., "agent-id.example.com")
  status: string;
  status_message?: string;
  created_by_id: string;
  last_heartbeat?: string;
  created_at: string;
  updated_at: string;
  // Phase 5: Agent Templates
  agent_type?: string;
  template_id?: string;
  startup_args?: string;
  env_vars?: Record<string, unknown>;
  ttl?: string;
  is_pinned?: boolean;
  cpu_cores?: number;
  memory_gib?: number;
  gpu_count?: number;
  gpu_model?: string;
  gpu_resource_name?: string;
  gpu_node_selector?: Record<string, string>;
}

export interface CreateServiceRequest {
  name: string;
  local_target?: string;
  external_port?: number;
  agent_id?: string;
  // Phase 5: Agent Templates
  template_id?: string;
  startup_args?: string;
  env_vars?: Record<string, unknown>;
  ttl?: string;
  workspace_id?: string; // Optional if passed via URL
  cpu_cores?: string;
  memory_gib?: string;
  gpu_count?: number;
  gpu_model?: string;
  gpu_resource_name?: string;
  gpu_node_selector?: Record<string, string>;
}

export interface UpdateServiceRequest {
  name?: string;
  local_target?: string;
  external_port?: number;
  agent_id?: string;
  status?: string;
  startup_args?: string;
  env_vars?: Record<string, unknown>;
  is_pinned?: boolean;
  cpu_cores?: string;
  memory_gib?: string;
  gpu_count?: number;
  gpu_model?: string;
  gpu_resource_name?: string;
  gpu_node_selector?: Record<string, string>;
  ttl?: string;
}

export interface AgentTemplate {
  id: string;
  name: string;
  agent_type: string;
  description?: string;
  docker_image: string;
  default_local_target: string;
  default_external_port: number;
  startup_args?: string;
  env_vars?: Record<string, unknown>;
  security_context?: Record<string, unknown>;
  volume_mounts?: unknown[];
  created_at: string;
  updated_at: string;
}

export interface SSHKey {
  id: string;
  name: string;
  public_key: string;
  fingerprint: string;
  added_at: string;
}

export interface CreateSSHKeyRequest {
  name: string;
  public_key: string;
}

// Audit Logs API
export const auditApi = {
  list: (params: AuditLogFilter) => {
    const query = new URLSearchParams({
      user_id: params.user_id || '',
      action: params.action || '',
      resource: params.resource || '',
      start_date: params.start_date || '',
      end_date: params.end_date || '',
      limit: params.limit?.toString() || '',
      offset: params.offset?.toString() || '',
    });
    // Remove empty parameters
    for (const [key, value] of Array.from(query.entries())) {
      if (!value) {
        query.delete(key);
      }
    }
    return api.get<{ logs: AuditLog[]; total: number }>(
      `/api/admin/audit-logs?${query.toString()}`,
    );
  },
};

export interface AuditLog {
  id: string;
  user_id: string;
  user?: User;
  action: string;
  resource: string;
  resource_id: string;
  old_data?: string;
  new_data?: string;
  timestamp: string;
}

export interface AuditLogFilter {
  user_id?: string;
  action?: string;
  resource?: string;
  start_date?: string;
  end_date?: string;
  limit?: number;
  offset?: number;
}

// Team Types (Multi-tenant)
export interface Team {
  id: number;
  name: string;
  display_name: string;
  namespace: string;
  status: 'active' | 'suspended';
  created_at: string;
  updated_at: string;
}

export interface TeamQuota {
  id: number;
  team_id: number;
  resource_config_id: number;
  quota: number;
  resource_config?: ResourceConfig;
  created_at: string;
  updated_at: string;
}

export interface TeamQuotaWithUsage extends TeamQuota {
  used?: number;
  resource_name?: string;
}

// Server response format for team quota (different from TeamQuota model)
export interface TeamQuotaItem {
  resource_config_id: number;
  resource_type: string;
  resource_name: string;
  display_name: string;
  quota: number;
  unit: string;
}

export interface CreateTeamRequest {
  name: string;
  display_name: string;
}

export interface UpdateTeamRequest {
  display_name?: string;
  status?: 'active' | 'suspended';
}

export interface UpdateTeamQuotaRequest {
  quotas: {
    resource_config_id: number;
    resource_type?: string;
    resource_name?: string;
    quota: number;
  }[];
}

// Teams API (admin only)
export const teamsApi = {
  list: () => api.get<{ teams: Team[] }>('/api/admin/teams').then((res) => res.teams),
  get: (id: number) =>
    api.get<{ team: Team; member_count: number }>(`/api/admin/teams/${id}`).then((res) => res.team),
  create: (team: CreateTeamRequest) => api.post<Team>('/api/admin/teams', team),
  update: (id: number, team: UpdateTeamRequest) => api.put<Team>(`/api/admin/teams/${id}`, team),
  delete: (id: number) => api.delete(`/api/admin/teams/${id}`),
  getMembers: (id: number) =>
    api.get<{ members: User[] }>(`/api/admin/teams/${id}/members`).then((res) => res.members),
  addMember: (teamId: number, userId: string) =>
    api.post(`/api/admin/teams/${teamId}/members`, { user_id: userId }),
  removeMember: (teamId: number, userId: string) =>
    api.delete(`/api/admin/teams/${teamId}/members/${userId}`),
  getQuota: (id: number) =>
    api
      .get<{ team: Team; quotas: TeamQuotaItem[] }>(`/api/teams/${id}/quota`)
      .then((res) => res.quotas),
  updateQuota: (id: number, request: UpdateTeamQuotaRequest) =>
    api.put<{ message: string }>(`/api/admin/teams/${id}/quota`, request),
};
