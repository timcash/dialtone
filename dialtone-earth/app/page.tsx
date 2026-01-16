import { Globe } from "@/components/globe"
import { ThemeToggle } from "@/components/theme-toggle"
import { Button } from "@/components/ui/button"
import { Github } from "lucide-react"
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
        <p className="text-lg md:text-xl text-muted-foreground max-w-md">robotic networks for earth</p>
      </div>

      {/* Theme toggle */}
      <div className="absolute top-4 right-4 z-20 flex items-center gap-2">
        <Button variant="ghost" size="icon" asChild>
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
