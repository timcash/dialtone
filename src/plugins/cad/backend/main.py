import cadquery as cq
import io
import os
import tempfile
import math
from fastapi import FastAPI, Response, Request
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

app = FastAPI()

# Enable CORS for local development
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Mock logger for gear_generator compatibility
class MockLogger:
    def info(self, msg): print(f"[INFO] {msg}")
    def error(self, msg): print(f"[ERROR] {msg}")
    def warn(self, msg): print(f"[WARN] {msg}")

logger = MockLogger()

def generate_gear(
    outer_diameter: float = 80.0,
    inner_diameter: float = 20.0,
    thickness: float = 8.0,
    tooth_height: float = 6.0,
    tooth_width: float = 4.0,
    num_teeth: int = 20,
    num_mounting_holes: int = 4,
    mounting_hole_diameter: float = 6.0
) -> bytes:
    """
    Generates a gear with the given parameters and returns the STL content as bytes.
    """
    logger.info(f"Generating gear with: OD={outer_diameter}, ID={inner_diameter}, Teeth={num_teeth}")
    
    try:
        # 1. Main body
        wheel = cq.Workplane("XY").cylinder(thickness, outer_diameter / 2.0)

        # 2. Inner hole
        wheel = wheel.faces(">Z").workplane().hole(inner_diameter)

        # 3. Teeth
        tooth = (cq.Workplane("XY")
                 .moveTo(outer_diameter / 2.0 - tooth_height, 0)
                 .lineTo(outer_diameter / 2.0, tooth_width / 2.0)
                 .lineTo(outer_diameter / 2.0, -tooth_width / 2.0)
                 .close()
                 .extrude(thickness))

        teeth = cq.Workplane("XY")
        for i in range(num_teeth):
            angle = i * 360.0 / num_teeth
            rotated_tooth = tooth.rotate((0, 0, 0), (0, 0, 1), angle)
            teeth = teeth.union(rotated_tooth)

        result = wheel.union(teeth)

        # 4. Mounting holes
        if num_mounting_holes > 0:
            mounting_hole_radius = inner_diameter / 2.0 + (outer_diameter/2.0 - inner_diameter/2.0) * 0.5

            base_hole = (cq.Workplane("XY")
                         .moveTo(mounting_hole_radius, 0)
                         .cylinder(thickness, mounting_hole_diameter / 2.0))
            
            mounting_holes = cq.Workplane("XY")
            for i in range(num_mounting_holes):
                angle = i * 360.0 / num_mounting_holes
                rotated_hole = base_hole.rotate((0, 0, 0), (0, 0, 1), angle)
                mounting_holes = mounting_holes.union(rotated_hole)
            
            result = result.cut(mounting_holes)

        with tempfile.NamedTemporaryFile(suffix=".stl", delete=False) as tmp:
            tmp_path = tmp.name
        
        cq.exporters.export(result, tmp_path)
        with open(tmp_path, 'rb') as f:
            stl_content = f.read()
        os.unlink(tmp_path)
        
        logger.info("Gear generation successful")
        return stl_content

    except Exception as e:
        logger.error(f"Failed to generate gear: {e}")
        raise e

class GearParams(BaseModel):
    outer_diameter: float = 80.0
    inner_diameter: float = 20.0
    thickness: float = 8.0
    tooth_height: float = 6.0
    tooth_width: float = 4.0
    num_teeth: int = 20
    num_mounting_holes: int = 4
    mounting_hole_diameter: float = 6.0

@app.post("/api/cad/generate")
async def post_generate(params: GearParams):
    stl_content = generate_gear(
        params.outer_diameter, params.inner_diameter, params.thickness,
        params.tooth_height, params.tooth_width, params.num_teeth,
        params.num_mounting_holes, params.mounting_hole_diameter
    )
    return Response(content=stl_content, media_type="application/sla")

@app.get("/api/cad")
async def get_cad(
    outer_diameter: float = 80.0,
    inner_diameter: float = 20.0,
    thickness: float = 8.0,
    tooth_height: float = 6.0,
    tooth_width: float = 4.0,
    num_teeth: int = 20,
    num_mounting_holes: int = 4,
    mounting_hole_diameter: float = 6.0
):
    source_code = ""
    try:
        # Instead of reading this file, we can read the reference file or just this one
        with open(__file__, "r") as f:
            source_code = f.read()
    except Exception as e:
        source_code = f"# Error reading source: {e}"

    return {
        "type": "gear",
        "parameters": {
            "outer_diameter": outer_diameter,
            "inner_diameter": inner_diameter,
            "thickness": thickness,
            "tooth_height": tooth_height,
            "tooth_width": tooth_width,
            "num_teeth": num_teeth,
            "num_mounting_holes": num_mounting_holes,
            "mounting_hole_diameter": mounting_hole_diameter
        },
        "source_code": source_code
    }

@app.get("/api/cad/download")
async def download_stl(
    outer_diameter: float = 80.0,
    inner_diameter: float = 20.0,
    thickness: float = 8.0,
    tooth_height: float = 6.0,
    tooth_width: float = 4.0,
    num_teeth: int = 20,
    num_mounting_holes: int = 4,
    mounting_hole_diameter: float = 6.0
):
    stl_content = generate_gear(
        outer_diameter, inner_diameter, thickness, tooth_height, 
        tooth_width, num_teeth, num_mounting_holes, mounting_hole_diameter
    )
    return Response(content=stl_content, media_type="application/sla", headers={
        "Content-Disposition": f"attachment; filename=gear_{num_teeth}t.stl"
    })

@app.get("/health")
async def health():
    return {"status": "ok"}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=8082)
