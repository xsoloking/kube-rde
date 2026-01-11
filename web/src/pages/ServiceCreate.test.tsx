import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import ServiceCreate from '../../pages/ServiceCreate';
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
      roles: ['developer'],
      enabled: true,
      created: Date.now(),
      ssh_keys: [
        {
          id: 'key-1',
          name: 'Test Key',
          public_key: 'ssh-rsa AAAA...',
          fingerprint: 'SHA256:...',
          added_at: '2024-01-01T00:00:00Z',
        },
      ],
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

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

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
    description: 'File server for file sharing',
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
];

const mockQuota: api.UserQuota = {
  user_id: 'test-user-id',
  cpu_cores: 8,
  memory_gi: 16,
  storage_quota: [{ name: 'standard', limit_gi: 100 }],
  gpu_quota: [
    {
      name: 'nvidia-a100',
      model_name: 'NVIDIA A100',
      limit: 2,
    },
  ],
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
};

const renderWithRouter = () => {
  return render(
    <MemoryRouter initialEntries={['/workspaces/workspace-001/services/create']}>
      <Routes>
        <Route path="/workspaces/:workspaceId/services/create" element={<ServiceCreate />} />
      </Routes>
    </MemoryRouter>,
  );
};

describe('ServiceCreate Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockClear();
    (api.userQuotaApi.get as any).mockResolvedValue(mockQuota);
    (api.agentTemplatesApi.list as any).mockResolvedValue(mockTemplates);
  });

  it('should load and display all 4 templates', async () => {
    renderWithRouter();

    // Wait for templates to load
    await waitFor(() => {
      expect(screen.getByText('OpenSSH Server')).toBeInTheDocument();
    });

    expect(screen.getByText('File Browser')).toBeInTheDocument();
    expect(screen.getByText('Code Server')).toBeInTheDocument();
    expect(screen.getByText('Jupyter Notebook')).toBeInTheDocument();
  });

  it('should show template selection step initially', async () => {
    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByText('Select Agent Type')).toBeInTheDocument();
    });

    expect(screen.getByText('Choose an agent type to get started')).toBeInTheDocument();
  });

  it('should transition to configuration step when template is selected', async () => {
    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByText('OpenSSH Server')).toBeInTheDocument();
    });

    // Click on SSH template - find button containing the template name
    const buttons = screen.getAllByRole('button');
    const sshTemplate = buttons.find((btn) => btn.textContent?.includes('OpenSSH Server'));
    expect(sshTemplate).toBeDefined();
    fireEvent.click(sshTemplate!);

    // Should show configuration form
    await waitFor(() => {
      expect(screen.getByText(/Configure your OpenSSH Server/)).toBeInTheDocument();
    });
  });

  it('should show resource configuration after template selection', async () => {
    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByText('OpenSSH Server')).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    const sshTemplate = buttons.find((btn) => btn.textContent?.includes('OpenSSH Server'));
    expect(sshTemplate).toBeDefined();
    fireEvent.click(sshTemplate!);

    // Should show resource configuration section
    await waitFor(() => {
      expect(screen.getByText(/Configure your OpenSSH Server/)).toBeInTheDocument();
    });

    // Check that resource sliders are present
    expect(screen.getByText(/CPU Cores/)).toBeInTheDocument();
    expect(screen.getByText(/Memory/)).toBeInTheDocument();
  });

  it('should allow adjusting resource configuration', async () => {
    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByText('OpenSSH Server')).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    const sshTemplate = buttons.find((btn) => btn.textContent?.includes('OpenSSH Server'));
    expect(sshTemplate).toBeDefined();
    fireEvent.click(sshTemplate!);

    await waitFor(() => {
      expect(screen.getByText(/Configure your OpenSSH Server/)).toBeInTheDocument();
    });

    // Check that CPU and Memory configuration is present
    const cpuSlider = screen.getAllByRole('slider')[0]; // First slider should be CPU
    expect(cpuSlider).toBeInTheDocument();

    // Adjust CPU cores
    fireEvent.change(cpuSlider, { target: { value: '6' } });
    expect(cpuSlider).toHaveValue('6');
  });

  it('should show Back button when template is selected', async () => {
    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByText('OpenSSH Server')).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    const sshTemplate = buttons.find((btn) => btn.textContent?.includes('OpenSSH Server'));
    expect(sshTemplate).toBeDefined();
    fireEvent.click(sshTemplate!);

    await waitFor(() => {
      const backButton = screen.getByRole('button', { name: /Back/ });
      expect(backButton).toBeInTheDocument();
    });
  });

  it('should show Create Service button only in configuration step', async () => {
    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByText('OpenSSH Server')).toBeInTheDocument();
    });

    // Should not have Create Service button initially
    expect(screen.queryByRole('button', { name: /Create Service/ })).not.toBeInTheDocument();

    // Click template
    const buttons = screen.getAllByRole('button');
    const sshTemplate = buttons.find((btn) => btn.textContent?.includes('OpenSSH Server'));
    expect(sshTemplate).toBeDefined();
    fireEvent.click(sshTemplate!);

    // Now should have Create Service button
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Create Service/ })).toBeInTheDocument();
    });
  });

  it('should display advanced options when expanded', async () => {
    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByText('OpenSSH Server')).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    const sshTemplate = buttons.find((btn) => btn.textContent?.includes('OpenSSH Server'));
    expect(sshTemplate).toBeDefined();
    fireEvent.click(sshTemplate!);

    await waitFor(() => {
      expect(screen.getByText('Advanced Options (Optional)')).toBeInTheDocument();
    });

    const advancedButton = screen.getByRole('button', { name: /Advanced Options/ });
    fireEvent.click(advancedButton);

    // Check that advanced options fields are now visible
    await waitFor(() => {
      expect(screen.getByText('Startup Arguments')).toBeInTheDocument();
      expect(screen.getByText(/Environment Variables/)).toBeInTheDocument();
    });
  });

  it('should enable Create Service button when template is selected', async () => {
    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByText('OpenSSH Server')).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    const sshTemplate = buttons.find((btn) => btn.textContent?.includes('OpenSSH Server'));
    expect(sshTemplate).toBeDefined();
    fireEvent.click(sshTemplate!);

    // Create button should be available after template selection
    await waitFor(() => {
      const createButton = screen.getByRole('button', { name: /Create Service/ });
      expect(createButton).toBeInTheDocument();
      expect(createButton).not.toBeDisabled();
    });
  });

  it('should handle API errors when creating service', async () => {
    (api.servicesApi.create as any).mockRejectedValueOnce(new Error('Failed to create service'));

    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByText('OpenSSH Server')).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    const sshTemplate = buttons.find((btn) => btn.textContent?.includes('OpenSSH Server'));
    expect(sshTemplate).toBeDefined();
    fireEvent.click(sshTemplate!);

    await waitFor(() => {
      expect(screen.getByText(/Configure your OpenSSH Server/)).toBeInTheDocument();
    });

    const createButton = await screen.findByRole('button', { name: /Create Service/ });
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByText(/Failed to create service/)).toBeInTheDocument();
    });
  });
});
