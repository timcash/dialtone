import { Button } from "@/components/ui/button"
import { Home as HomeIcon, BookOpen, Shield, Zap, Cpu, Globe } from "lucide-react"
import Link from "next/link"

export default function AboutPage() {
    return (
        <main className="relative min-h-screen flex flex-col items-center py-20 px-4 bg-background">
            <div className="max-w-3xl w-full space-y-12 relative z-10">
                <section className="text-left space-y-4">
                    <h1 className="text-5xl font-bold tracking-tight text-primary font-[family-name:var(--font-space-grotesk)]">
                        Vision
                    </h1>
                    <p className="text-xl text-muted-foreground leading-relaxed">
                        Dialtone is aspirationally a <strong>robotic video operations network</strong> designed to allow humans and AI to cooperatively train and operate thousands of robots simultaneously with low latency.
                    </p>
                </section>

                <section className="grid gap-8 md:grid-cols-2">
                    <div className="p-6 rounded-2xl border bg-card/50 backdrop-blur-sm space-y-3">
                        <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center text-primary">
                            <Globe className="h-6 w-6" />
                        </div>
                        <h3 className="text-xl font-bold">Network-First</h3>
                        <p className="text-muted-foreground">
                            Prioritizing secure, low-latency communication between distributed components through a unified mesh network.
                        </p>
                    </div>
                    <div className="p-6 rounded-2xl border bg-card/50 backdrop-blur-sm space-y-3">
                        <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center text-primary">
                            <Cpu className="h-6 w-6" />
                        </div>
                        <h3 className="text-xl font-bold">Hardware Agnostic</h3>
                        <p className="text-muted-foreground">
                            A single software binary that can run on any device—from Raspberry Pis to factory controllers.
                        </p>
                    </div>
                    <div className="p-6 rounded-2xl border bg-card/50 backdrop-blur-sm space-y-3">
                        <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center text-primary">
                            <Shield className="h-6 w-6" />
                        </div>
                        <h3 className="text-xl font-bold">Secure Connectivity</h3>
                        <p className="text-muted-foreground">
                            Integrated mesh VPN and zero-config peer-to-peer connectivity even behind restrictive NATs.
                        </p>
                    </div>
                    <div className="p-6 rounded-2xl border bg-card/50 backdrop-blur-sm space-y-3">
                        <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center text-primary">
                            <Zap className="h-6 w-6" />
                        </div>
                        <h3 className="text-xl font-bold">Real-time Operations</h3>
                        <p className="text-muted-foreground">
                            Low-latency video streaming and telemetry for collaborative human-AI robot control.
                        </p>
                    </div>
                </section>

                <section className="space-y-6">
                    <h2 className="text-3xl font-bold font-[family-name:var(--font-space-grotesk)]">Join the Mission</h2>
                    <div className="prose prose-gray dark:prose-invert max-w-none">
                        <p>
                            Dialtone is an open project with an ambitious goal. We are looking for robot builders to integrate their hardware, AI researchers to deploy models, and developers to help us build the most accessible robotic network on Earth.
                        </p>
                    </div>
                </section>

                <footer className="pt-10 flex gap-4">
                    <Button variant="outline" asChild className="rounded-full">
                        <Link href="/">
                            <HomeIcon className="mr-2 h-4 w-4" />
                            Home
                        </Link>
                    </Button>
                    <Button variant="default" asChild className="rounded-full">
                        <Link href="/docs">
                            <BookOpen className="mr-2 h-4 w-4" />
                            Documentation
                        </Link>
                    </Button>
                </footer>
            </div>
        </main>
    )
}
