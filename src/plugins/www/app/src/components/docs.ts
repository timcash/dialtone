export function mountDocs(container: HTMLElement) {
  container.innerHTML = `
        <div class="page-content">
            <h1>Documentation</h1>
            <p class="lead">Get started with Dialtone and explore the technical specifications of the robotic network.
            </p>

            <h2>
                <div class="inline-flex items-center gap-2">
                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none"
                        stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <polyline points="4 17 10 11 4 5" />
                        <line x1="12" y1="19" x2="20" y2="19" />
                    </svg>
                    Quick Start
                </div>
            </h2>
            <div class="code-block">
                <p><span class="comment"># Clone the repo</span></p>
                <p>git clone https://github.com/timcash/dialtone.git</p>
                <p>export DIALTONE_ENV="~/dialtone_env"</p>
                <br>
                <p><span class="comment"># Bootstrap environment</span></p>
                <p>./setup.sh</p>
                <br>
                <p><span class="comment"># Install dependencies</span></p>
                <p>./dialtone.sh install --linux-wsl</p>
                <br>
                <p><span class="comment"># Build and Start</span></p>
                <p>./dialtone.sh build --local</p>
                <p>./dialtone.sh start --local</p>
            </div>

            <h2>WWW Development</h2>
            <p class="text-muted-foreground">Run the public site locally, validate it with browser tests, then build/publish.</p>
            <div class="code-block">
                <p><span class="comment"># Start the local dev server</span></p>
                <p>./dialtone.sh www dev</p>
                <br>
                <p><span class="comment"># Run browser integration tests (captures console logs + JS exceptions)</span></p>
                <p>./dialtone.sh plugin test www</p>
                <br>
                <p><span class="comment"># Build and publish</span></p>
                <p>./dialtone.sh www build</p>
                <p>./dialtone.sh www publish</p>
                <br>
                <p><span class="comment"># Inspect deployments</span></p>
                <p>./dialtone.sh www logs &lt;deployment-url-or-id&gt;</p>
                <p>./dialtone.sh www domain [deployment-url]</p>
            </div>

            <div class="grid-2">
                <div class="card">
                    <div class="card-header-icon">
                        <svg class="text-primary" xmlns="http://www.w3.org/2000/svg" width="24" height="24"
                            viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
                            stroke-linecap="round" stroke-linejoin="round">
                            <path
                                d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" />
                            <polyline points="3.27 6.96 12 12.01 20.73 6.96" />
                            <line x1="12" y1="22.08" x2="12" y2="12" />
                        </svg>
                        <h3>Tech Stack</h3>
                    </div>
                    <ul>
                        <li>• Lang: <strong>Go</strong> (Concurrency, Type-safety)</li>
                        <li>• Messaging: <strong>NATS</strong> Bus</li>
                        <li>• Networking: <strong>Tailscale</strong> Mesh VPN</li>
                        <li>• Protocol: <strong>MAVLink</strong> & WebRTC</li>
                        <li>• Targets: Linux, ARM64 (Raspi), macOS</li>
                    </ul>
                </div>
                <div class="card">
                    <div class="card-header-icon">
                        <svg class="text-primary" xmlns="http://www.w3.org/2000/svg" width="24" height="24"
                            viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
                            stroke-linecap="round" stroke-linejoin="round">
                            <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
                        </svg>
                        <h3>Features</h3>
                    </div>
                    <ul>
                        <li>• V4L2 Camera Support</li>
                        <li>• Geospatial (Google Earth Engine)</li>
                        <li>• System-Tuned AI Assistant</li>
                        <li>• Telemetry Streaming & Storage</li>
                        <li>• "Digital Twin" Simulation</li>
                    </ul>
                </div>
            </div>

            <section style="margin-top: 3rem; margin-bottom: 3rem;">
                <h2>
                    <div class="inline-flex items-center gap-2">
                        <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none"
                            stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z" />
                            <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z" />
                        </svg>
                        Vendor Docs
                    </div>
                </h2>
                <p class="text-muted-foreground"><em>No vendor documentation currently available.</em></p>
            </section>

            <h2>CLI Reference</h2>
            <p>The <code>dialtone.sh</code> tool is your primary interface for management:</p>
            <ul>
                <li><code>install</code>: Set up a local-user dev environment.</li>
                <li><code>build</code>: Create production-ready binaries.</li>
                <li><code>deploy</code>: Push updates to remote robots via SSH.</li>
                <li><code>issue</code>: Manage project tasks and feedback.</li>
            </ul>
        </div>
    `;
  return {
    dispose: () => {
      container.innerHTML = '';
    },
    setVisible: (visible: boolean) => {
      container.style.opacity = visible ? '1' : '0';
    }
  };
}
