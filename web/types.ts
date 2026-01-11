export interface User {
  id: string;
  name: string;
  email: string;
  avatar: string;
  role: 'Admin' | 'Developer' | 'Viewer';
  created: string;
  status: 'Active' | 'Pending' | 'Disabled';
  cpuUsage: number;
  ramUsage: number;
}

export interface Workspace {
  id: string;
  name: string;
  description: string;
  status: 'Normal' | 'Abnormal' | 'Provisioning';
  storageUsed: number;
  storageAllocated: number;
  runningServices: number;
  created: string;
  type: string;
}

export interface Service {
  id: string;
  name: string;
  type: 'NodePort' | 'ClusterIP' | 'LoadBalancer';
  status: 'Running' | 'Error' | 'Starting' | 'Stopped';
  endpoint: string;
  age: string;
  icon: string;
}
