import { User, Workspace, Service } from './types';

export const MOCK_USERS: User[] = [
  {
    id: 'u1',
    name: 'alex.dev',
    email: 'alex@kuberde.io',
    avatar:
      'https://lh3.googleusercontent.com/aida-public/AB6AXuCJJOfTXiMKneBZdgwLYi6bB8IrtHgurdWZpWryWLVAsxJA-RzeuMMYclYmwQnK4AnQsejdV13lZwyo7FtuhUScQZo_YEzjl41KDuOO4JOius-_v__nQ36QGTnakPYga27TXLUdvcFIy3O76Fo08d5Za-uN2WSaU8t2ly9eCPj1Nr__OqNR_O-iFtuG7sEz21T6TsnrljJ2_PFKWgkrIS0XPHvo7rgQ266FRS3kCZFMCa4tgdb9vfurbwPaSY092Nyy7HSzxxZoQiaT',
    role: 'Developer',
    created: 'Oct 24, 2023',
    status: 'Active',
    cpuUsage: 45,
    ramUsage: 60,
  },
  {
    id: 'u2',
    name: 'sarah.jenkins',
    email: 'sarah.j@kuberde.io',
    avatar:
      'https://lh3.googleusercontent.com/aida-public/AB6AXuAVjbEdEu2y76yAkTa_aSAH9pyvvEXhzhM6qaVnnn2OrSBEsJXi1qwhslxQklccQ34wEZzWwF7D_lc_EcfP5JbcBSNDeQ7h7AalEFy8QT88UplojdSy56QAgzGt6Jl8pOjm_ZSbpllwjPDyETg-K08pQs30pvL60JOau2jjlbQ9-3033nj4vdOJGA49st7yNY6cHoRbEk9Q2AUx4NB6DLm7SCFQu-K5GLhW38fXlHz1LOsnZD1z7XGA_Uf9Noo-U2IuoNaTzhMW4SzN',
    role: 'Admin',
    created: 'Sep 12, 2023',
    status: 'Active',
    cpuUsage: 12,
    ramUsage: 24,
  },
  {
    id: 'u3',
    name: 'mike.k',
    email: 'mike.k@vendor.com',
    avatar:
      'https://lh3.googleusercontent.com/aida-public/AB6AXuC0TdVgWHe_kteJeUqoO9wEeAvHKn0VaWCJsgZzif7hnDaHKAH2YXbxpDv-Ji20XFup9ezHkAbV057xgxyjgE4HraEgQdilmTlf8ZIphFd2jphYWgEr6UgCfvCC6odFpW3x3w8YnS6dZX1qf1u8gBTuGLuAC1HRzAHjXqy_2ayV4ieMS8FRqO5q3RY7PQQiMRDB0qZuk67ghuUPujfks8riJOiZJJQI_t5WHs83OgWEVb5kajT64Ma7vABPCwB5q1Q0O5_Mr-iK8LU7',
    role: 'Viewer',
    created: 'Nov 02, 2023',
    status: 'Pending',
    cpuUsage: 0,
    ramUsage: 0,
  },
];

export const MOCK_WORKSPACES: Workspace[] = [
  {
    id: 'ws1',
    name: 'Frontend-Dev-Alpha',
    description: 'Primary environment for React frontend development. Includes Node v18.',
    status: 'Normal',
    storageUsed: 12,
    storageAllocated: 50,
    runningServices: 5,
    created: '2 days ago',
    type: 'dns',
  },
  {
    id: 'ws2',
    name: 'Backend-API-Staging',
    description: 'Integration testing environment for the Python payment gateway.',
    status: 'Abnormal',
    storageUsed: 48,
    storageAllocated: 50,
    runningServices: 2,
    created: '5 days ago',
    type: 'warning',
  },
];

export const MOCK_SERVICES: Service[] = [
  {
    id: 's1',
    name: 'JupyterLab',
    type: 'NodePort',
    status: 'Running',
    endpoint: 'http://10.0.0.1:8888',
    age: '2 days',
    icon: 'terminal',
  },
  {
    id: 's2',
    name: 'VS Code Server',
    type: 'ClusterIP',
    status: 'Error',
    endpoint: 'Not exposed externally',
    age: '5 hours',
    icon: 'code',
  },
];
