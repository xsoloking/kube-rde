/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { agentTemplatesApi, servicesApi, AgentTemplate } from '../../services/api';

describe('Agent Templates API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (global.fetch as any).mockClear();
  });

  describe('agentTemplatesApi', () => {
    it('should fetch all templates', async () => {
      const mockTemplates: AgentTemplate[] = [
        {
          id: 'tpl-ssh-001',
          name: 'OpenSSH Server',
          agent_type: 'ssh',
          description: 'SSH Server Template',
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
          description: 'File Server Template',
          docker_image: 'filebrowser/filebrowser:latest',
          default_local_target: 'localhost:3000',
          default_external_port: 9000,
          startup_args: '',
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
        },
      ];

      (global.fetch as any).mockResolvedValueOnce({
        ok: true,
        json: async () => mockTemplates,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
      });

      const templates = await agentTemplatesApi.list();
      expect(templates).toEqual(mockTemplates);
      expect(templates.length).toBe(2);
    });

    it('should fetch a single template by ID', async () => {
      const mockTemplate: AgentTemplate = {
        id: 'tpl-ssh-001',
        name: 'OpenSSH Server',
        agent_type: 'ssh',
        description: 'SSH Server Template',
        docker_image: 'linuxserver/openssh-server:latest',
        default_local_target: 'localhost:22',
        default_external_port: 22,
        startup_args: '',
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      };

      (global.fetch as any).mockResolvedValueOnce({
        ok: true,
        json: async () => mockTemplate,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
      });

      const template = await agentTemplatesApi.get('tpl-ssh-001');
      expect(template).toEqual(mockTemplate);
      expect(template.agent_type).toBe('ssh');
    });

    it('should delete a template', async () => {
      (global.fetch as any).mockResolvedValueOnce({
        ok: true,
        json: async () => ({}),
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
      });

      await agentTemplatesApi.delete('custom-tpl-001');
      expect(global.fetch).toHaveBeenCalled();
    });

    it('should handle API errors gracefully', async () => {
      (global.fetch as any).mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: async () => 'Internal Server Error',
        headers: new Headers({ 'content-type': 'text/plain' }),
      });

      await expect(agentTemplatesApi.list()).rejects.toThrow();
    });
  });

  describe('servicesApi with templates', () => {
    it('should create a service with template', async () => {
      const mockService = {
        id: 'service-001',
        name: 'My SSH Service',
        local_target: 'localhost:22',
        external_port: 2022,
        workspace_id: 'workspace-001',
        status: 'pending',
        created_by_id: 'user-001',
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
        template_id: 'tpl-ssh-001',
        agent_type: 'ssh',
        startup_args: '',
        env_vars: {},
      };

      (global.fetch as any).mockResolvedValueOnce({
        ok: true,
        json: async () => mockService,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
      });

      const service = await servicesApi.create('workspace-001', {
        name: 'My SSH Service',
        template_id: 'tpl-ssh-001',
        local_target: 'localhost:22',
        external_port: 2022,
      });

      expect(service).toEqual(mockService);
      expect(service.template_id).toBe('tpl-ssh-001');
      expect(service.agent_type).toBe('ssh');
    });

    it('should include env_vars in service creation', async () => {
      const mockService = {
        id: 'service-002',
        name: 'My Coder Service',
        local_target: 'localhost:8080',
        external_port: 8080,
        workspace_id: 'workspace-001',
        status: 'pending',
        created_by_id: 'user-001',
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
        template_id: 'tpl-coder-001',
        agent_type: 'coder',
        startup_args: '--bind 0.0.0.0:8080',
        env_vars: { PASSWORD: 'secret123' },
      };

      (global.fetch as any).mockResolvedValueOnce({
        ok: true,
        json: async () => mockService,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
      });

      const service = await servicesApi.create('workspace-001', {
        name: 'My Coder Service',
        template_id: 'tpl-coder-001',
        env_vars: { PASSWORD: 'secret123' },
      });

      expect(service.env_vars).toEqual({ PASSWORD: 'secret123' });
    });
  });
});
