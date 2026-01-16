"use client"

import { useEffect, useRef } from "react"
import * as d3 from "d3"
import { useTheme } from "next-themes"
import { feature } from "topojson-client"

export function Globe() {
  const svgRef = useRef<SVGSVGElement>(null)
  const { resolvedTheme } = useTheme()

  useEffect(() => {
    if (!svgRef.current) return

    const svg = d3.select(svgRef.current)
    svg.selectAll("*").remove()

    const width = svgRef.current.clientWidth
    const height = svgRef.current.clientHeight

    const projection = d3
      .geoOrthographic()
      .scale(Math.min(width, height) / 2.2)
      .center([0, 0])
      .rotate([0, -30])
      .translate([width / 2, height / 2])

    const path = d3.geoPath().projection(projection)

    const strokeColor = resolvedTheme === "dark" ? "#ffffff" : "#000000"

    // Globe outline circle
    svg
      .append("circle")
      .attr("fill", "none")
      .attr("stroke", strokeColor)
      .attr("stroke-width", 0.5)
      .attr("stroke-opacity", 0.2)
      .attr("cx", width / 2)
      .attr("cy", height / 2)
      .attr("r", projection.scale())

    const map = svg.append("g")

    const dotsGroup = svg.append("g").attr("class", "dots")

    // Add graticule (grid lines)
    const graticule = d3.geoGraticule().step([20, 20])

    map
      .append("path")
      .datum(graticule)
      .attr("class", "graticule")
      .attr("d", path as any)
      .attr("fill", "none")
      .attr("stroke", strokeColor)
      .attr("stroke-width", 0.3)
      .attr("stroke-opacity", 0.15)

    const createRandomDot = () => {
      const lat = Math.random() * 180 - 90 // -90 to 90
      const lon = Math.random() * 360 - 180 // -180 to 180
      const coords: [number, number] = [lon, lat]

      // Project the coordinates
      const projected = projection(coords)
      if (!projected) return

      // Check if point is visible (on front of globe)
      const geoDistance = d3.geoDistance(coords, [-projection.rotate()[0], -projection.rotate()[1]])
      if (geoDistance > Math.PI / 2) return // Point is on back of globe

      const size = 2 + Math.random() * 6

      const baseDuration = 2000 + (size / 8) * 2000
      const fadeInDuration = 300 + Math.random() * 200
      const fadeOutDuration = baseDuration * 0.4

      const dot = dotsGroup
        .append("circle")
        .datum(coords)
        .attr("cx", projected[0])
        .attr("cy", projected[1])
        .attr("r", 0) // Start at 0 and grow
        .attr("fill", "#1e40af") // Dark blue
        .attr("opacity", 0)

      dot
        .transition()
        .duration(fadeInDuration)
        .attr("opacity", 0.7 + Math.random() * 0.3)
        .attr("r", size)
        .transition()
        .duration(baseDuration - fadeInDuration - fadeOutDuration)
        .transition()
        .duration(fadeOutDuration)
        .attr("opacity", 0)
        .attr("r", size * 0.5)
        .remove()
    }

    let dotIntervalId: ReturnType<typeof setInterval>
    dotIntervalId = setInterval(() => {
      // Random chance to create 0-2 dots per tick for natural variation
      const dotCount = Math.random() < 0.8 ? 1 : Math.random() < 0.5 ? 0 : 2
      for (let i = 0; i < dotCount; i++) {
        createRandomDot()
      }
    }, 200)

    // Load world data
    d3.json("https://cdn.jsdelivr.net/npm/world-atlas@2/land-110m.json").then((data: any) => {
      if (!data) return

      const land = feature(data, data.objects.land)

      map
        .append("g")
        .attr("class", "countries")
        .selectAll("path")
        .data(land.features || [land])
        .enter()
        .append("path")
        .attr("d", (d: any) => path(d))
        .attr("fill", "none")
        .attr("stroke", strokeColor)
        .attr("stroke-width", 0.5)
        .attr("stroke-opacity", 0.4)

      // Slow rotation animation
      let rotation = 0
      const timer = d3.timer(() => {
        rotation += 0.15
        projection.rotate([rotation, -30])
        map.selectAll("path").attr("d", path as any)

        dotsGroup.selectAll("circle").each(function (d: any) {
          const projected = projection(d)
          if (projected) {
            const geoDistance = d3.geoDistance(d, [-projection.rotate()[0], -projection.rotate()[1]])
            d3.select(this)
              .attr("cx", projected[0])
              .attr("cy", projected[1])
              .attr("visibility", geoDistance > Math.PI / 2 ? "hidden" : "visible")
          }
        })
      })

      return () => timer.stop()
    })

    const handleResize = () => {
      if (!svgRef.current) return
      const newWidth = svgRef.current.clientWidth
      const newHeight = svgRef.current.clientHeight

      projection.scale(Math.min(newWidth, newHeight) / 2.2).translate([newWidth / 2, newHeight / 2])

      svg
        .select("circle")
        .attr("cx", newWidth / 2)
        .attr("cy", newHeight / 2)
        .attr("r", projection.scale())

      map.selectAll("path").attr("d", path as any)
    }

    window.addEventListener("resize", handleResize)
    return () => {
      window.removeEventListener("resize", handleResize)
      clearInterval(dotIntervalId) // Cleanup dot interval
    }
  }, [resolvedTheme])

  return <svg ref={svgRef} className="absolute inset-0 w-full h-full" style={{ opacity: 0.6 }} />
}
