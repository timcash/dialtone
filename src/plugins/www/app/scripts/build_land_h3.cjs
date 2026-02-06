const fs = require("fs/promises");
const path = require("path");
const { polygonToCells } = require("h3-js");

const resolution = Number.parseInt(process.argv[2] || "3", 10);
if (Number.isNaN(resolution) || resolution < 0 || resolution > 7) {
  console.error("Resolution must be an integer between 0 and 7.");
  process.exit(1);
}

const appRoot = path.resolve(__dirname, "..");
const geojsonPath = path.join(appRoot, "public", "land.geojson");
const outputPath = path.join(appRoot, "public", "land.h3.json");

const readGeojson = async () => {
  const raw = await fs.readFile(geojsonPath, "utf8");
  return JSON.parse(raw);
};

const geojsonToCells = (geojson) => {
  const cells = new Set();
  if (!geojson?.features) return [];
  geojson.features.forEach((feature) => {
    const geometry = feature?.geometry;
    if (!geometry) return;
    const polygons =
      geometry.type === "Polygon"
        ? [geometry.coordinates]
        : geometry.type === "MultiPolygon"
          ? geometry.coordinates
          : [];
    polygons.forEach((coords) => {
      try {
        polygonToCells(coords, resolution, true).forEach((cell) => cells.add(cell));
      } catch {
        // Skip invalid polygons.
      }
    });
  });
  return Array.from(cells);
};

const run = async () => {
  const geojson = await readGeojson();
  const cells = geojsonToCells(geojson);
  if (cells.length === 0) {
    console.error("No H3 cells generated.");
    process.exit(1);
  }
  const payload = {
    resolution,
    cells,
    createdAt: new Date().toISOString(),
    source: "land.geojson",
  };
  await fs.writeFile(outputPath, JSON.stringify(payload));
  console.log(`Wrote ${cells.length.toLocaleString()} cells to ${outputPath}`);
};

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
