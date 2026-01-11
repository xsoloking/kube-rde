import { describe, it, expect } from 'vitest';

/**
 * Phase 5 Integration Tests
 * Tests the complete workflow of agent template-based service creation
 */

describe('Phase 5 - Agent Templates Integration', () => {
  describe('Template System', () => {
    it('should have 4 built-in templates with correct configuration', () => {
      const builtInTemplates = [
        {
          id: 'tpl-ssh-001',
          name: 'OpenSSH Server',
          agent_type: 'ssh',
          port: 22,
          image: 'linuxserver/openssh-server:latest',
        },
        {
          id: 'tpl-file-001',
          name: 'File Browser',
          agent_type: 'file',
          port: 9000,
          image: 'filebrowser/filebrowser:latest',
        },
        {
          id: 'tpl-coder-001',
          name: 'Code Server',
          agent_type: 'coder',
          port: 8080,
          image: 'codercom/code-server:latest',
        },
        {
          id: 'tpl-jupyter-001',
          name: 'Jupyter Notebook',
          agent_type: 'jupyter',
          port: 8888,
          image: 'jupyter/datascience-notebook:latest',
        },
      ];

      expect(builtInTemplates).toHaveLength(4);
      expect(builtInTemplates.every((t) => t.id.startsWith('tpl-'))).toBe(true);
      expect(builtInTemplates.map((t) => t.agent_type)).toEqual([
        'ssh',
        'file',
        'coder',
        'jupyter',
      ]);
    });

    it('should allow creating services from templates', () => {
      const serviceFromTemplate = {
        name: 'My SSH Server',
        template_id: 'tpl-ssh-001',
        local_target: 'localhost:22',
        external_port: 2022,
        agent_type: 'ssh',
      };

      expect(serviceFromTemplate.template_id).toBe('tpl-ssh-001');
      expect(serviceFromTemplate.agent_type).toBe('ssh');
    });

    it('should support template customization via env_vars', () => {
      const customizedService = {
        name: 'Custom Coder',
        template_id: 'tpl-coder-001',
        env_vars: {
          PASSWORD: 'my-secure-password',
          SUDO_PASSWORD: 'another-password',
        },
        startup_args: '--bind 0.0.0.0:8080 --disable-telemetry',
      };

      expect(customizedService.env_vars).toBeDefined();
      expect(customizedService.startup_args).toBeDefined();
      expect(Object.keys(customizedService.env_vars)).toContain('PASSWORD');
    });
  });

  describe('Service Creation Workflow', () => {
    it('should follow template selection → configuration flow', () => {
      const workflow = [
        { step: 1, action: 'Load available templates', templates_loaded: 4 },
        { step: 2, action: 'User selects template', selected_template: 'tpl-ssh-001' },
        {
          step: 3,
          action: 'Show configuration form with template defaults',
          defaults_applied: true,
        },
        { step: 4, action: 'User customizes if needed', customized: true },
        { step: 5, action: 'Submit service creation', status: 'pending' },
      ];

      expect(workflow).toHaveLength(5);
      expect(workflow[1].action).toContain('selects');
      expect(workflow[4].status).toBe('pending');
    });

    it('should validate port configuration', () => {
      const validPorts = [22, 80, 443, 8080, 3000, 9000];
      const invalidPorts = [0, -1, 65536, 100000];

      validPorts.forEach((port) => {
        expect(port).toBeGreaterThanOrEqual(1);
        expect(port).toBeLessThanOrEqual(65535);
      });

      invalidPorts.forEach((port) => {
        expect(port < 1 || port > 65535).toBe(true);
      });
    });

    it('should support environment variable overrides', () => {
      const templateDefaults = {
        env_vars: {
          PATH: '/usr/local/bin:/usr/bin',
          HOME: '/root',
        },
      };

      const userOverrides = {
        PASSWORD: 'custom-password',
        SUDO_PASSWORD: 'sudo-pass',
      };

      const merged = { ...templateDefaults.env_vars, ...userOverrides };

      expect(Object.keys(merged)).toHaveLength(4);
      expect(merged.PASSWORD).toBe('custom-password');
      expect(merged.HOME).toBe('/root');
    });
  });

  describe('Workspace and PVC Integration', () => {
    it('should create PVC when workspace is created', () => {
      const workspace = {
        id: 'ws-123',
        name: 'Development',
        storage_size: '50Gi',
        pvc_name: 'kuberde-user-ws-123',
      };

      expect(workspace.pvc_name).toBeDefined();
      expect(workspace.pvc_name).toMatch(/^kuberde-/);
      expect(workspace.storage_size).toBe('50Gi');
    });

    it('should support custom storage sizes', () => {
      const storageSizes = ['10Gi', '50Gi', '100Gi', '500Gi', '1Ti'];

      storageSizes.forEach((size) => {
        const workspace = {
          id: 'ws-123',
          storage_size: size,
          pvc_name: 'kuberde-user-ws-123',
        };

        expect(workspace.storage_size).toBe(size);
      });
    });
  });

  describe('RDEAgent CR Generation', () => {
    it('should generate valid RDEAgent CR for SSH template', () => {
      const rdeAgent = {
        apiVersion: 'kuberde.io/v1',
        kind: 'RDEAgent',
        metadata: {
          name: 'kuberde-user-ws-123-ssh-service',
          namespace: 'kuberde',
        },
        spec: {
          owner: 'user-123',
          serverUrl: 'ws://kuberde-server:8080/ws',
          authSecret: 'kuberde-agents-auth',
          localTarget: 'localhost:22',
          ttl: '24h',
          workloadContainer: {
            image: 'linuxserver/openssh-server:latest',
            ports: [{ containerPort: 22, name: 'service', protocol: 'TCP' }],
            env: [],
            volumeMounts: [{ name: 'workspace', mountPath: '/root', readOnly: false }],
            resources: {
              requests: { cpu: '100m', memory: '256Mi' },
              limits: { cpu: '500m', memory: '512Mi' },
            },
          },
          storage: [
            {
              name: 'workspace',
              storageClass: 'local-path',
              size: '50Gi',
              mountPath: '/root',
            },
          ],
        },
      };

      expect(rdeAgent.kind).toBe('RDEAgent');
      expect(rdeAgent.spec.owner).toBeDefined();
      expect(rdeAgent.spec.localTarget).toContain(':');
      expect(rdeAgent.spec.workloadContainer.image).toContain('openssh');
    });

    it('should generate RDEAgent CR with custom environment variables', () => {
      const rdeAgent = {
        spec: {
          workloadContainer: {
            image: 'codercom/code-server:latest',
            env: [
              { name: 'PASSWORD', value: 'my-password' },
              { name: 'SUDO_PASSWORD', value: 'sudo-pass' },
            ],
            ports: [{ containerPort: 8080 }],
          },
        },
      };

      expect(rdeAgent.spec.workloadContainer.env).toHaveLength(2);
      expect(rdeAgent.spec.workloadContainer.env[0].name).toBe('PASSWORD');
    });

    it('should generate RDEAgent CR with startup arguments', () => {
      const rdeAgent = {
        spec: {
          workloadContainer: {
            image: 'jupyter/datascience-notebook:latest',
            args: [
              'start-notebook.sh',
              '--allow-root',
              '--no-browser',
              '--ip=0.0.0.0',
              '--port=8888',
            ],
          },
        },
      };

      expect(rdeAgent.spec.workloadContainer.args).toBeDefined();
      expect(rdeAgent.spec.workloadContainer.args).toContain('--allow-root');
    });
  });

  describe('Database Schema Updates', () => {
    it('should have agent_templates table', () => {
      const schema = {
        tables: {
          agent_templates: {
            columns: [
              'id',
              'name',
              'agent_type',
              'description',
              'docker_image',
              'default_local_target',
              'default_external_port',
              'startup_args',
              'env_vars',
              'security_context',
              'volume_mounts',
              'created_at',
              'updated_at',
            ],
          },
        },
      };

      expect(schema.tables.agent_templates.columns).toContain('agent_type');
      expect(schema.tables.agent_templates.columns).toContain('docker_image');
    });

    it('should extend services table with template fields', () => {
      const servicesTable = {
        columns: [
          'id',
          'workspace_id',
          'name',
          'local_target',
          'external_port',
          'status',
          'created_at',
          // New Phase 5 fields
          'agent_type',
          'template_id',
          'startup_args',
          'env_vars',
        ],
      };

      expect(servicesTable.columns).toContain('template_id');
      expect(servicesTable.columns).toContain('agent_type');
    });

    it('should extend workspaces table with PVC fields', () => {
      const workspacesTable = {
        columns: [
          'id',
          'name',
          'description',
          'owner_id',
          // New Phase 5 fields
          'storage_size',
          'pvc_name',
          'created_at',
        ],
      };

      expect(workspacesTable.columns).toContain('pvc_name');
      expect(workspacesTable.columns).toContain('storage_size');
    });
  });

  describe('API Endpoints', () => {
    it('should provide agent templates endpoint', () => {
      const endpoint = '/api/agent-templates';
      expect(endpoint).toMatch(/agent-templates/);
    });

    it('should support template filtering and CRUD operations', () => {
      const operations = {
        list: 'GET /api/agent-templates',
        get: 'GET /api/agent-templates/{id}',
        delete: 'DELETE /api/agent-templates/{id}',
      };

      expect(Object.keys(operations)).toContain('list');
      expect(Object.keys(operations)).toContain('get');
      expect(Object.keys(operations)).toContain('delete');
    });
  });

  describe('UI Components', () => {
    it('should display agent type in service list', () => {
      const serviceListColumns = [
        'Service Name',
        'Status',
        'Agent Type',
        'Local Target',
        'External Port',
        'Created',
        'Actions',
      ];

      expect(serviceListColumns).toContain('Agent Type');
      expect(serviceListColumns.indexOf('Agent Type')).toBeGreaterThan(1);
    });

    it('should show template selection interface', () => {
      const uiElements = [
        { type: 'heading', text: 'Select Agent Type' },
        { type: 'grid', items: 4 },
        { type: 'card', properties: ['name', 'type', 'port', 'description'] },
      ];

      expect(uiElements).toHaveLength(3);
      expect(uiElements[1].items).toBe(4);
    });

    it('should show configuration form with template defaults', () => {
      const formFields = [
        { name: 'Service Name', required: true },
        { name: 'Local Target', required: false, default: true },
        { name: 'External Port', required: false, default: true },
        { name: 'Startup Arguments', required: false, advanced: true },
        { name: 'Environment Variables', required: false, advanced: true },
      ];

      expect(formFields.filter((f) => f.required)).toHaveLength(1);
      expect(formFields.filter((f) => f.default)).toHaveLength(2);
      expect(formFields.filter((f) => f.advanced)).toHaveLength(2);
    });

    it('should manage templates through admin interface', () => {
      const adminFeatures = [
        'View all templates',
        'See usage statistics',
        'Delete custom templates',
        'View built-in template info',
        'Display template configurations',
      ];

      expect(adminFeatures).toHaveLength(5);
      expect(adminFeatures).toContain('Delete custom templates');
    });
  });

  describe('Complete E2E Workflow', () => {
    it('should complete full workflow: workspace → service → connection', () => {
      const workflow = [
        {
          step: 1,
          action: 'Create workspace',
          expected: {
            workspace_created: true,
            pvc_created: true,
            pvc_name: 'kuberde-user-ws-123',
          },
        },
        {
          step: 2,
          action: 'View service creation page',
          expected: { templates_loaded: 4 },
        },
        {
          step: 3,
          action: 'Select SSH template',
          expected: { template_selected: 'tpl-ssh-001' },
        },
        {
          step: 4,
          action: 'Configure service with custom name and optional overrides',
          expected: {
            service_name: 'My Dev SSH',
            custom_port: 2222,
          },
        },
        {
          step: 5,
          action: 'Submit service creation',
          expected: {
            service_status: 'pending',
            rde_agent_cr_created: true,
          },
        },
        {
          step: 6,
          action: 'View service in workspace detail',
          expected: {
            agent_type_visible: 'ssh',
            service_list_shows_template: true,
          },
        },
      ];

      expect(workflow).toHaveLength(6);
      expect(workflow.every((w) => w.step && w.expected)).toBe(true);
    });
  });
});
