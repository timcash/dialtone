import { Button } from "@/components/ui/button"
import { Home as HomeIcon, Info, Terminal, Box, Activity, Github } from "lucide-react"
import Link from "next/link"

export default function DocsPage() {
    return (
        <main className="relative min-h-screen flex flex-col items-center py-20 px-4 bg-background">
            <div className="max-w-4xl w-full space-y-12 relative z-10">
                <header className="space-y-4">
                    <h1 className="text-5xl font-bold tracking-tight text-primary font-[family-name:var(--font-space-grotesk)]">
                        Documentation
                    </h1>
                    <p className="text-xl text-muted-foreground">
                        Get started with Dialtone and explore the technical specifications of the robotic network.
                    </p>
                </header>

                <section className="space-y-6">
                    <h2 className="text-3xl font-bold flex items-center gap-2 font-[family-name:var(--font-space-grotesk)]">
                        <Terminal className="h-8 w-8" /> Quick Start
                    </h2>
                    <div className="bg-zinc-950 text-zinc-50 p-6 rounded-2xl font-mono text-sm overflow-x-auto border border-zinc-800 shadow-2xl">
                        <div className="space-y-2">
                            <p className="text-zinc-500"># Clone the repo</p>
                            <p>git clone https://github.com/timcash/dialtone.git</p>
                            <p>export DAILTONE_ENV="~/dialtone_env"</p>
                            <br />
                            <p className="text-zinc-500"># Bootstrap environment</p>
                            <p>./setup.sh</p>
                            <br />
                            <p className="text-zinc-500"># Install dependencies</p>
                            <p>go run dialtone-dev install --linux-wsl</p>
                            <br />
                            <p className="text-zinc-500"># Build and Start</p>
                            <p>go run dialtone-dev build --local</p>
                            <p>go run ./bin/dialtone start --local</p>
                        </div>
                    </div>
                </section>

                <section className="grid gap-6 md:grid-cols-2">
                    <div className="p-6 rounded-2xl border bg-card space-y-4">
                        <div className="flex items-center gap-3">
                            <Box className="h-6 w-6 text-primary" />
                            <h3 className="text-xl font-bold">Tech Stack</h3>
                        </div>
                        <ul className="space-y-2 text-muted-foreground">
                            <li>• Lang: <strong>Go</strong> (Concurrency, Type-safety)</li>
                            <li>• Messaging: <strong>NATS</strong> Bus</li>
                            <li>• Networking: <strong>Tailscale</strong> Mesh VPN</li>
                            <li>• Protocol: <strong>MAVLink</strong> & WebRTC</li>
                            <li>• Targets: Linux, ARM64 (Raspi), macOS</li>
                        </ul>
                    </div>
                    <div className="p-6 rounded-2xl border bg-card space-y-4">
                        <div className="flex items-center gap-3">
                            <Activity className="h-6 w-6 text-primary" />
                            <h3 className="text-xl font-bold">Features</h3>
                        </div>
                        <ul className="space-y-2 text-muted-foreground">
                            <li>• V4L2 Camera Support</li>
                            <li>• Geospatial (Google Earth Engine)</li>
                            <li>• System-Tuned AI Assistant</li>
                            <li>• Telemetry Streaming & Storage</li>
                            <li>• "Digital Twin" Simulation</li>
                        </ul>
                    </div>
                </section>

                <section className="space-y-4">
                    <h2 className="text-2xl font-bold font-[family-name:var(--font-space-grotesk)]">CLI Reference</h2>
                    <div className="prose prose-gray dark:prose-invert max-w-none">
                        <p>
                            The <code>dialtone-dev.go</code> tool is your primary interface for management:
                        </p>
                        <ul>
                            <li><code>install</code>: Set up a local-user dev environment.</li>
                            <li><code>build</code>: Create production-ready binaries.</li>
                            <li><code>deploy</code>: Push updates to remote robots via SSH.</li>
                            <li><code>issue</code>: Manage project tasks and feedback.</li>
                        </ul>
                    </div>
                </section>

                <footer className="pt-10 flex gap-4">
                    <Button variant="outline" asChild className="rounded-full">
                        <Link href="/">
                            <HomeIcon className="mr-2 h-4 w-4" />
                            Home
                        </Link>
                    </Button>
                    <Button variant="outline" asChild className="rounded-full">
                        <Link href="/about">
                            <Info className="mr-2 h-4 w-4" />
                            About
                        </Link>
                    </Button>
                    <Button variant="default" asChild className="rounded-full">
                        <Link href="https://github.com/timcash/dialtone" target="_blank">
                            <Github className="mr-2 h-4 w-4" />
                            GitHub
                        </Link>
                    </Button>
                </footer>
            </div>
        </main>
    )
}
