"use client"

import { useEffect, useRef } from "react"
import * as THREE from "three"

export function LineGraph() {
    const containerRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        if (!containerRef.current) return

        const width = containerRef.current.clientWidth
        const height = containerRef.current.clientHeight

        const scene = new THREE.Scene()
        const camera = new THREE.PerspectiveCamera(75, width / height, 0.1, 1000)
        camera.position.z = 50

        const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true })
        renderer.setSize(width, height)
        renderer.setPixelRatio(window.devicePixelRatio)
        containerRef.current.appendChild(renderer.domElement)

        const points: THREE.Vector3[] = []
        const lineCount = 50
        const pointCount = 20

        const lineMaterials = Array.from({ length: lineCount }, (_, i) => {
            return new THREE.LineBasicMaterial({
                color: new THREE.Color().setHSL(0.5 + Math.random() * 0.2, 0.7, 0.5),
                transparent: true,
                opacity: 0.3 + Math.random() * 0.4,
            })
        })

        const lines: THREE.Line[] = []
        const lineData: { velocities: THREE.Vector3[]; offsets: number[] }[] = []

        for (let i = 0; i < lineCount; i++) {
            const geometry = new THREE.BufferGeometry()
            const initialPoints: number[] = []
            const velocities: THREE.Vector3[] = []
            const offsets: number[] = []

            const xBase = (Math.random() - 0.5) * 100
            const yBase = (Math.random() - 0.5) * 60

            for (let j = 0; j < pointCount; j++) {
                initialPoints.push(xBase + j * 2, yBase + (Math.random() - 0.5) * 10, (Math.random() - 0.5) * 10)
                velocities.push(new THREE.Vector3((Math.random() - 0.5) * 0.1, (Math.random() - 0.5) * 0.1, (Math.random() - 0.5) * 0.1))
                offsets.push(Math.random() * Math.PI * 2)
            }

            geometry.setAttribute("position", new THREE.Float32BufferAttribute(initialPoints, 3))
            const line = new THREE.Line(geometry, lineMaterials[i])
            scene.add(line)
            lines.push(line)
            lineData.push({ velocities, offsets })
        }

        let frame = 0
        const animate = () => {
            frame += 0.01
            requestAnimationFrame(animate)

            lines.forEach((line, i) => {
                const positions = line.geometry.attributes.position.array as Float32Array
                const data = lineData[i]

                for (let j = 0; j < pointCount; j++) {
                    const idx = j * 3
                    // Movement code
                    positions[idx] += data.velocities[j].x
                    positions[idx + 1] += Math.sin(frame + data.offsets[j]) * 0.05 + data.velocities[j].y
                    positions[idx + 2] += data.velocities[j].z

                    // Simple boundary check
                    if (Math.abs(positions[idx]) > 100) data.velocities[j].x *= -1
                    if (Math.abs(positions[idx + 1]) > 60) data.velocities[j].y *= -1
                }
                line.geometry.attributes.position.needsUpdate = true
            })

            renderer.render(scene, camera)
        }

        animate()

        const handleResize = () => {
            if (!containerRef.current) return
            const w = containerRef.current.clientWidth
            const h = containerRef.current.clientHeight
            camera.aspect = w / h
            camera.updateProjectionMatrix()
            renderer.setSize(w, h)
        }

        window.addEventListener("resize", handleResize)

        return () => {
            window.removeEventListener("resize", handleResize)
            if (containerRef.current) {
                containerRef.current.removeChild(renderer.domElement)
            }
            renderer.dispose()
        }
    }, [])

    return <div ref={containerRef} className="absolute inset-0 w-full h-full" />
}
