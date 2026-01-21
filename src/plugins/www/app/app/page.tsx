"use client"

import { useEffect, useRef, useState } from "react"
import { Globe } from "@/components/globe"
import { LineGraph } from "@/components/line-graph"
import { ThemeToggle } from "@/components/theme-toggle"
import { Button } from "@/components/ui/button"
import { Github, Home as HomeIcon, Info, BookOpen } from "lucide-react"
import Link from "next/link"

function LazyVideo({ src }: { src: string }) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const [isVisible, setIsVisible] = useState(false)

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            setIsVisible(true)
            observer.unobserve(entry.target)
          }
        })
      },
      { threshold: 0.1 },
    )

    if (videoRef.current) {
      observer.observe(videoRef.current)
    }

    return () => observer.disconnect()
  }, [])

  return (
    <video
      ref={videoRef}
      className="absolute inset-0 w-full h-full object-cover"
      autoPlay
      loop
      muted
      playsInline
      src={isVisible ? src : undefined}
    />
  )
}

export default function Home() {
  return (
    <main className="snap-container bg-background">
      {/* Navigation - Fixed across all slides */}
      <nav className="fixed top-4 left-1/2 -translate-x-1/2 z-50 flex items-center gap-1 px-2 py-1 rounded-full border bg-background/50 backdrop-blur-md">
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
      <div className="fixed top-4 right-4 z-50 flex items-center gap-2">
        <Button variant="ghost" size="icon" asChild className="rounded-full">
          <Link href="https://github.com/timcash/dialtone" target="_blank" rel="noopener noreferrer">
            <Github className="h-5 w-5" />
            <span className="sr-only">GitHub</span>
          </Link>
        </Button>
        <ThemeToggle />
      </div>

      {/* Slide 1: Globe */}
      <article className="snap-slide flex items-center justify-center">
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
          <p className="text-sm text-muted-foreground mt-2">v1.0.1</p>
        </div>
      </article>

      {/* Slide 2: Video */}
      <article className="snap-slide flex items-center justify-center">
        <LazyVideo src="/video1.mp4" />
        <div className="absolute inset-0 bg-black/40 z-10" />
        <div className="relative z-20 text-center px-4">
          <h2 className="text-4xl md:text-6xl font-bold text-white mb-4 font-[family-name:var(--font-space-grotesk)]">
            Robotic Operations
          </h2>
          <p className="text-lg md:text-xl text-white/80 max-w-2xl mx-auto">
            Low-latency video streaming and control for cooperative human-AI systems.
          </p>
        </div>
      </article>

      {/* Slide 3: Three.js Line Graph */}
      <article className="snap-slide flex items-center justify-center">
        <LineGraph />
        <div className="relative z-10 text-center px-4">
          <h2 className="text-4xl md:text-6xl font-bold text-foreground mb-4 font-[family-name:var(--font-space-grotesk)]">
            Neural Connectivity
          </h2>
          <p className="text-lg md:text-xl text-muted-foreground max-w-2xl mx-auto">
            Decentralized networking patterns connecting edge nodes across the globe.
          </p>
        </div>
      </article>
    </main>
  )
}
