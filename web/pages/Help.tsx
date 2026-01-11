import React, { useState } from 'react';
import { Link } from 'react-router-dom';

const Help: React.FC = () => {
  const [activeSection, setActiveSection] = useState('getting-started');

  const scrollToSection = (id: string) => {
    setActiveSection(id);
    const element = document.getElementById(id);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth' });
    }
  };

  const sections = [
    { id: 'getting-started', title: 'Getting Started', icon: 'rocket_launch' },
    { id: 'connectivity', title: 'Connectivity', icon: 'cable' },
    { id: 'core-concepts', title: 'Core Concepts', icon: 'school' },
    { id: 'resources', title: 'Resources & GPU', icon: 'memory' },
    { id: 'faq', title: 'FAQ', icon: 'help' },
  ];

  return (
    <div className="flex h-full animate-fade-in">
      {/* Sidebar Navigation */}
      <div className="w-64 bg-surface-dark border-r border-border-dark p-6 flex-shrink-0 overflow-y-auto hidden lg:block">
        <h2 className="text-xl font-bold text-white mb-6 flex items-center gap-2">
          <span className="material-symbols-outlined text-primary">menu_book</span>
          Documentation
        </h2>
        <nav className="space-y-1">
          {sections.map((section) => (
            <button
              key={section.id}
              onClick={() => scrollToSection(section.id)}
              className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg transition-all duration-200 text-sm font-medium ${
                activeSection === section.id
                  ? 'bg-primary/10 text-primary border border-primary/20'
                  : 'text-text-secondary hover:text-white hover:bg-white/5'
              }`}
            >
              <span className="material-symbols-outlined text-[20px]">{section.icon}</span>
              {section.title}
            </button>
          ))}
        </nav>
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-y-auto p-8 lg:p-12 scroll-smooth">
        <div className="max-w-4xl mx-auto space-y-12 pb-24">
          <div className="flex flex-col gap-2 border-b border-border-dark pb-8">
            <h1 className="text-4xl font-bold tracking-tight text-white">Help Center</h1>
            <p className="text-text-secondary text-lg">
              Guides, references, and troubleshooting for the KubeRDE platform.
            </p>
          </div>

          {/* 1. Getting Started */}
          <section id="getting-started" className="space-y-6 scroll-mt-24">
            <div className="flex items-center gap-3">
              <div className="size-10 rounded-lg bg-primary/20 flex items-center justify-center text-primary">
                <span className="material-symbols-outlined text-2xl">rocket_launch</span>
              </div>
              <h2 className="text-2xl font-bold text-white">Getting Started</h2>
            </div>

            <div className="bg-surface-dark border border-border-dark rounded-xl p-6 space-y-4">
              <h3 className="text-lg font-bold text-white">1. Create a Workspace</h3>
              <p className="text-text-secondary">
                A <strong>Workspace</strong> is your project folder. It holds your persistent
                storage (PVC) and groups your services. Go to the{' '}
                <Link to="/workspaces" className="text-primary hover:underline">
                  Workspaces
                </Link>{' '}
                page and click "Create New Workspace".
              </p>
            </div>

            <div className="bg-surface-dark border border-border-dark rounded-xl p-6 space-y-4">
              <h3 className="text-lg font-bold text-white">2. Launch a Service</h3>
              <p className="text-text-secondary">
                Inside your workspace, click "Create Service". Choose a template:
              </p>
              <ul className="list-disc list-inside text-text-secondary space-y-2 ml-2">
                <li>
                  <strong className="text-white">SSH Server:</strong> Standard Linux environment.
                  Best for VS Code Remote.
                </li>
                <li>
                  <strong className="text-white">Code Server:</strong> VS Code running in your
                  browser.
                </li>
                <li>
                  <strong className="text-white">Jupyter:</strong> Python data science environment.
                </li>
              </ul>
            </div>
          </section>

          {/* 2. Connectivity */}
          <section id="connectivity" className="space-y-6 scroll-mt-24">
            <div className="flex items-center gap-3">
              <div className="size-10 rounded-lg bg-emerald-500/20 flex items-center justify-center text-emerald-500">
                <span className="material-symbols-outlined text-2xl">cable</span>
              </div>
              <h2 className="text-2xl font-bold text-white">Connectivity</h2>
            </div>

            <div className="bg-surface-dark border border-border-dark rounded-xl p-6 space-y-4">
              <h3 className="text-lg font-bold text-white">Step 1: Download CLI</h3>
              <p className="text-text-secondary">
                You need the <code>kuberde-cli</code> to authenticate and proxy connections.
              </p>
              <div className="flex gap-3 mt-2">
                <a
                  href="/download/cli/darwin-arm64"
                  className="px-4 py-2 bg-background-dark border border-border-dark rounded-lg text-sm font-mono hover:border-primary transition-colors"
                >
                  macOS (Apple Silicon)
                </a>
                <a
                  href="/download/cli/darwin-amd64"
                  className="px-4 py-2 bg-background-dark border border-border-dark rounded-lg text-sm font-mono hover:border-primary transition-colors"
                >
                  macOS (Intel)
                </a>
                <a
                  href="/download/cli/linux-amd64"
                  className="px-4 py-2 bg-background-dark border border-border-dark rounded-lg text-sm font-mono hover:border-primary transition-colors"
                >
                  Linux (AMD64)
                </a>
                <a
                  href="/download/cli/windows-amd64.exe"
                  className="px-4 py-2 bg-background-dark border border-border-dark rounded-lg text-sm font-mono hover:border-primary transition-colors"
                >
                  Windows
                </a>
              </div>
            </div>

            <div className="bg-surface-dark border border-border-dark rounded-xl p-6 space-y-4">
              <h3 className="text-lg font-bold text-white">Step 2: Login via CLI</h3>
              <div className="bg-black/30 p-4 rounded-lg border border-border-dark">
                <code className="text-emerald-400 font-mono text-sm block">
                  ./kuberde-cli login --issuer https://sso.byai.uk/realms/kuberde
                </code>
              </div>
            </div>

            <div className="bg-surface-dark border border-border-dark rounded-xl p-6 space-y-4">
              <h3 className="text-lg font-bold text-white">Step 3: Connect via SSH</h3>
              <p className="text-text-secondary">
                Add this to your <code>~/.ssh/config</code> file:
              </p>
              <div className="bg-black/30 p-4 rounded-lg border border-border-dark">
                <pre className="text-blue-300 font-mono text-xs leading-relaxed">
                  {`Host kuberde-dev
    User root
    ProxyCommand /path/to/kuberde-cli connect wss://frp.byai.uk/connect/YOUR_AGENT_ID
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null`}
                </pre>
              </div>
              <p className="text-xs text-text-secondary">
                Replace <code>YOUR_AGENT_ID</code> with the ID found on your service card (e.g.,{' '}
                <code>user-alice-dev-box</code>).
              </p>
            </div>
          </section>

          {/* 3. Core Concepts */}
          <section id="core-concepts" className="space-y-6 scroll-mt-24">
            <div className="flex items-center gap-3">
              <div className="size-10 rounded-lg bg-blue-500/20 flex items-center justify-center text-blue-500">
                <span className="material-symbols-outlined text-2xl">school</span>
              </div>
              <h2 className="text-2xl font-bold text-white">Core Concepts</h2>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="bg-surface-dark border border-border-dark rounded-xl p-6">
                <h3 className="font-bold text-white mb-2 flex items-center gap-2">
                  <span className="material-symbols-outlined text-primary">folder</span>
                  Workspace
                </h3>
                <p className="text-sm text-text-secondary leading-relaxed">
                  A logic grouping of your work. Each workspace has a dedicated{' '}
                  <strong>Persistent Volume (PVC)</strong>. All services within a workspace share
                  the same <code>/home/workspace</code> directory, allowing you to switch between VS
                  Code, Jupyter, and SSH without moving files.
                </p>
              </div>
              <div className="bg-surface-dark border border-border-dark rounded-xl p-6">
                <h3 className="font-bold text-white mb-2 flex items-center gap-2">
                  <span className="material-symbols-outlined text-purple-500">deployed_code</span>
                  Service (Agent)
                </h3>
                <p className="text-sm text-text-secondary leading-relaxed">
                  An ephemeral computing environment (Kubernetes Pod). It runs your tools (Python,
                  Go, Node.js). Services can be stopped and started. <strong>Note:</strong> Files
                  outside of <code>/home/workspace</code>
                  are lost when the service stops.
                </p>
              </div>
            </div>
          </section>

          {/* 4. Resources */}
          <section id="resources" className="space-y-6 scroll-mt-24">
            <div className="flex items-center gap-3">
              <div className="size-10 rounded-lg bg-orange-500/20 flex items-center justify-center text-orange-500">
                <span className="material-symbols-outlined text-2xl">memory</span>
              </div>
              <h2 className="text-2xl font-bold text-white">Resources & GPU</h2>
            </div>

            <div className="bg-surface-dark border border-border-dark rounded-xl p-6 space-y-4">
              <h3 className="text-lg font-bold text-white">Resource Quotas</h3>
              <p className="text-text-secondary">
                Each user has a global quota for CPU cores, Memory (GiB), and GPUs. You cannot
                create new services if you exceed your allocated quota. Stop unused services to free
                up resources.
              </p>
            </div>

            <div className="bg-surface-dark border border-border-dark rounded-xl p-6 space-y-4">
              <h3 className="text-lg font-bold text-white">Auto-Scaling (TTL)</h3>
              <p className="text-text-secondary">
                To save resources, services have a <strong>Time To Live (TTL)</strong> setting
                (e.g., 8 hours). If no active connection (SSH or Web) is detected for the TTL
                duration, the service will automatically stop (scale to zero). Connecting to it
                again will automatically wake it up (takes ~30 seconds).
              </p>
            </div>
          </section>

          {/* 5. FAQ */}
          <section id="faq" className="space-y-6 scroll-mt-24">
            <div className="flex items-center gap-3">
              <div className="size-10 rounded-lg bg-gray-500/20 flex items-center justify-center text-gray-400">
                <span className="material-symbols-outlined text-2xl">help</span>
              </div>
              <h2 className="text-2xl font-bold text-white">FAQ</h2>
            </div>

            <div className="space-y-4">
              <details className="group bg-surface-dark border border-border-dark rounded-xl open:border-primary/50 transition-all">
                <summary className="flex items-center justify-between p-6 cursor-pointer font-bold text-white">
                  Why is my service status "Pending"?
                  <span className="material-symbols-outlined group-open:rotate-180 transition-transform">
                    expand_more
                  </span>
                </summary>
                <div className="px-6 pb-6 text-text-secondary text-sm leading-relaxed border-t border-border-dark/50 pt-4">
                  "Pending" usually means the cluster is scaling up nodes or pulling the docker
                  image. If it stays pending for more than 5 minutes, you might have exceeded your
                  resource quota or the cluster is full.
                </div>
              </details>

              <details className="group bg-surface-dark border border-border-dark rounded-xl open:border-primary/50 transition-all">
                <summary className="flex items-center justify-between p-6 cursor-pointer font-bold text-white">
                  I lost my files! Where are they?
                  <span className="material-symbols-outlined group-open:rotate-180 transition-transform">
                    expand_more
                  </span>
                </summary>
                <div className="px-6 pb-6 text-text-secondary text-sm leading-relaxed border-t border-border-dark/50 pt-4">
                  Remember that only files inside <code>/home/workspace</code> are persistent. Files
                  in the system root (like installed packages via <code>apt-get</code>) are reset
                  when the service restarts. Use custom Dockerfiles or scripts to install packages
                  on startup.
                </div>
              </details>

              <details className="group bg-surface-dark border border-border-dark rounded-xl open:border-primary/50 transition-all">
                <summary className="flex items-center justify-between p-6 cursor-pointer font-bold text-white">
                  How do I reset my environment?
                  <span className="material-symbols-outlined group-open:rotate-180 transition-transform">
                    expand_more
                  </span>
                </summary>
                <div className="px-6 pb-6 text-text-secondary text-sm leading-relaxed border-t border-border-dark/50 pt-4">
                  You can click the "Restart" button on the service page. This will delete the
                  current Pod and create a fresh one. Your data in <code>/home/workspace</code> will
                  remain safe.
                </div>
              </details>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
};

export default Help;
