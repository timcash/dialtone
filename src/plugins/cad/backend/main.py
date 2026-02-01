import cadquery as cq
import io
import os
import sys
import tempfile
import math
import argparse
import json

# Mock logger for gear_generator compatibility
class MockLogger:
    def info(self, msg): print(f"[INFO] {msg}", file=sys.stderr)
    def error(self, msg): print(f"[ERROR] {msg}", file=sys.stderr)
    def warn(self, msg): print(f"[WARN] {msg}", file=sys.stderr)

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

def main():
    parser = argparse.ArgumentParser(description="Generate a parametric gear STL.")
    parser.add_argument("--outer_diameter", type=float, default=80.0)
    parser.add_argument("--inner_diameter", type=float, default=20.0)
    parser.add_argument("--thickness", type=float, default=8.0)
    parser.add_argument("--tooth_height", type=float, default=6.0)
    parser.add_argument("--tooth_width", type=float, default=4.0)
    parser.add_argument("--num_teeth", type=int, default=20)
    parser.add_argument("--num_mounting_holes", type=int, default=4)
    parser.add_argument("--mounting_hole_diameter", type=float, default=6.0)
    parser.add_argument("--output", type=str, help="Output file path. If not provided, writes to stdout.")
    
    args = parser.parse_args()

    try:
        stl_content = generate_gear(
            args.outer_diameter, args.inner_diameter, args.thickness,
            args.tooth_height, args.tooth_width, args.num_teeth,
            args.num_mounting_holes, args.mounting_hole_diameter
        )
        
        if args.output:
            with open(args.output, 'wb') as f:
                f.write(stl_content)
        else:
            sys.stdout.buffer.write(stl_content)
            
    except Exception as e:
        logger.error(f"Execution failed: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
