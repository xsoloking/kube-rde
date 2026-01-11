import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import AgentTemplates from '../../pages/AgentTemplates';
import * as api from '../../services/api';

// Mock the API
vi.mock('../../services/api');

// Mock the AuthContext
vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: {
      id: 'test-user-id',
      username: 'testuser',
      email: 'test@example.com',
      roles: ['admin'],
      enabled: true,
      created: Date.now(),
    },
    loading: false,
    isAuthenticated: true,
    login: vi.fn(),
    logout: vi.fn(),
    refreshToken: vi.fn(),
    refreshUser: vi.fn(),
  }),
  AuthProvider: ({ children }: { children: React.ReactNode }) => children,
}));

const mockTemplates: api.AgentTemplate[] = [
  {
    id: 'tpl-ssh-001',
    name: 'OpenSSH Server',
    agent_type: 'ssh',
    description: 'SSH Server for remote access',
    docker_image: 'linuxserver/openssh-server:latest',
    default_local_target: 'localhost:22',
    default_external_port: 22,
    startup_args: '',
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 'tpl-file-001',
    name: 'File Browser',
    agent_type: 'file',
    description: 'File server',
    docker_image: 'filebrowser/filebrowser:latest',
    default_local_target: 'localhost:3000',
    default_external_port: 9000,
    startup_args: '',
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 'tpl-coder-001',
    name: 'Code Server',
    agent_type: 'coder',
    description: 'Web-based VS Code',
    docker_image: 'codercom/code-server:latest',
    default_local_target: 'localhost:8080',
    default_external_port: 8080,
    startup_args: '',
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 'tpl-jupyter-001',
    name: 'Jupyter Notebook',
    agent_type: 'jupyter',
    description: 'Data science notebook',
    docker_image: 'jupyter/datascience-notebook:latest',
    default_local_target: 'localhost:8888',
    default_external_port: 8888,
    startup_args: '',
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 'custom-tpl-001',
    name: 'Custom Template',
    agent_type: 'ssh',
    description: 'Custom user template',
    docker_image: 'myimage:latest',
    default_local_target: 'localhost:2222',
    default_external_port: 2222,
    startup_args: '',
    created_at: '2024-01-02T00:00:00Z',
    updated_at: '2024-01-02T00:00:00Z',
  },
];

describe('AgentTemplates Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.agentTemplatesApi.list as any).mockResolvedValue(mockTemplates);
  });

  it('should render page title and description', async () => {
    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    expect(screen.getByText('Agent Templates')).toBeInTheDocument();
    expect(screen.getByText(/Manage agent templates/)).toBeInTheDocument();
  });

  it('should display template statistics', async () => {
    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Total Templates')).toBeInTheDocument();
    });

    expect(screen.getByText('Built-In')).toBeInTheDocument();
    expect(screen.getByText('Custom')).toBeInTheDocument();

    // Should show correct counts
    await waitFor(() => {
      const totalCount = screen.getByText('5');
      expect(totalCount).toBeInTheDocument();
    });
  });

  it('should load and display all templates in table', async () => {
    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('OpenSSH Server')).toBeInTheDocument();
    });

    expect(screen.getByText('File Browser')).toBeInTheDocument();
    expect(screen.getByText('Code Server')).toBeInTheDocument();
    expect(screen.getByText('Jupyter Notebook')).toBeInTheDocument();
    expect(screen.getByText('Custom Template')).toBeInTheDocument();
  });

  it('should display table headers correctly', async () => {
    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('OpenSSH Server')).toBeInTheDocument();
    });

    expect(screen.getByText('Template')).toBeInTheDocument();
    expect(screen.getByText('Type')).toBeInTheDocument();
    expect(screen.getByText('Docker Image')).toBeInTheDocument();
    expect(screen.getByText('Default Port')).toBeInTheDocument();
    expect(screen.getByText('Actions')).toBeInTheDocument();
  });

  it('should show agent type badges with correct styling', async () => {
    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    await waitFor(() => {
      const sshBadges = screen.getAllByText('ssh');
      expect(sshBadges.length).toBeGreaterThan(0);
    });

    expect(screen.getByText('file')).toBeInTheDocument();
    expect(screen.getByText('coder')).toBeInTheDocument();
    expect(screen.getByText('jupyter')).toBeInTheDocument();
  });

  it('should show "Built-in" label for built-in templates', async () => {
    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    await waitFor(() => {
      const builtInLabels = screen.getAllByText('Built-in');
      expect(builtInLabels.length).toBeGreaterThan(0);
    });
  });

  it('should show delete button only for custom templates', async () => {
    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Custom Template')).toBeInTheDocument();
    });

    // Find delete buttons - should only be one (for custom template)
    const deleteButtons = screen
      .getAllByRole('button')
      .filter((btn) => btn.getAttribute('title') === 'Delete template');

    expect(deleteButtons.length).toBe(1);
  });

  it('should allow deleting custom templates with confirmation', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValueOnce(true);
    (api.agentTemplatesApi.delete as any).mockResolvedValueOnce({});

    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Custom Template')).toBeInTheDocument();
    });

    // Find and click delete button for custom template
    const deleteButtons = screen
      .getAllByRole('button')
      .filter((btn) => btn.getAttribute('title') === 'Delete template');

    fireEvent.click(deleteButtons[0]);

    expect(confirmSpy).toHaveBeenCalled();
    expect(confirmSpy.mock.calls[0][0]).toContain('delete');

    confirmSpy.mockRestore();
  });

  it('should reload templates after deletion', async () => {
    vi.spyOn(window, 'confirm').mockReturnValueOnce(true);
    (api.agentTemplatesApi.delete as any).mockResolvedValueOnce({});

    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Custom Template')).toBeInTheDocument();
    });

    const deleteButtons = screen
      .getAllByRole('button')
      .filter((btn) => btn.getAttribute('title') === 'Delete template');

    fireEvent.click(deleteButtons[0]);

    // Wait for delete call and reload
    await waitFor(() => {
      expect(api.agentTemplatesApi.delete as any).toHaveBeenCalled();
    });
  });

  it('should display error message when loading fails', async () => {
    (api.agentTemplatesApi.list as any).mockRejectedValue(new Error('Network error'));

    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument();
    });
  });

  it('should display empty state when no templates', async () => {
    (api.agentTemplatesApi.list as any).mockResolvedValue([]);

    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('No templates found')).toBeInTheDocument();
    });
  });

  it('should show info banner about built-in protection', async () => {
    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Built-in templates are protected')).toBeInTheDocument();
    });

    expect(screen.getByText(/Built-in templates.*cannot be deleted/)).toBeInTheDocument();
  });

  it('should display docker image and port information', async () => {
    render(
      <BrowserRouter>
        <AgentTemplates />
      </BrowserRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('OpenSSH Server')).toBeInTheDocument();
    });

    expect(screen.getByText('linuxserver/openssh-server:latest')).toBeInTheDocument();
    expect(screen.getByText('filebrowser/filebrowser:latest')).toBeInTheDocument();

    // Check ports
    const portNumbers = screen.getAllByText('22');
    expect(portNumbers.length).toBeGreaterThan(0);
  });
});
