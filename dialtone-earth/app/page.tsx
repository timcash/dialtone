import { Globe } from "@/components/globe"
import { ThemeToggle } from "@/components/theme-toggle"
import { Button } from "@/components/ui/button"
import { Github, Home as HomeIcon, Info, BookOpen } from "lucide-react"
import Link from "next/link"

export default function Home() {
  return (
    <main className="relative min-h-screen flex flex-col items-center justify-center overflow-hidden bg-background">
      {/* Globe background */}
      <div className="absolute inset-0 flex items-center justify-center">
        <div className="relative w-full h-full max-w-3xl max-h-3xl aspect-square">
          <Globe />
        </div>
      </div>

      <div className="relative z-10 text-left px-4">
        <h1 className="text-5xl md:text-7xl font-bold tracking-tight text-foreground mb-4 font-[family-name:var(--font-space-grotesk)]">
          dialtone.earth
        </h1>
        <p className="text-lg md:text-xl text-muted-foreground max-w-md">unified robotic networks for earth</p>
      </div>

      {/* Navigation */}
      <nav className="absolute top-4 left-1/2 -translate-x-1/2 z-20 flex items-center gap-1 px-2 py-1 rounded-full border bg-background/50 backdrop-blur-md">
        <Button variant="ghost" size="icon" asChild className="rounded-full">
          <Link href="/">
            <HomeIcon className="h-5 w-5" />
            <span className="sr-only">Home</span>
          </Link>
        </Button>
        <Button variant="ghost" size="icon" asChild className="rounded-full">
          <Link href="/about">
            <Info className="h-5 w-5" />
            <span className="sr-only">About</span>
          </Link>
        </Button>
        <Button variant="ghost" size="icon" asChild className="rounded-full">
          <Link href="/docs">
            <BookOpen className="h-5 w-5" />
            <span className="sr-only">Docs</span>
          </Link>
        </Button>
      </nav>

      {/* Action Buttons & Theme toggle */}
      <div className="absolute top-4 right-4 z-20 flex items-center gap-2">
        <Button variant="ghost" size="icon" asChild className="rounded-full">
          <Link href="https://github.com/timcash/dialtone" target="_blank" rel="noopener noreferrer">
            <Github className="h-5 w-5" />
            <span className="sr-only">GitHub</span>
          </Link>
        </Button>
        <ThemeToggle />
      </div>
    </main>
  )
}
