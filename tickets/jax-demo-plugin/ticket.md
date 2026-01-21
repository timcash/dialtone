# Branch: jax-demo-plugin
# Task: Create JAX Demo Plugin with Geospatial Theme

> IMPORTANT: Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh plugin create jax-demo` to create the plugin structure.

## Goals
1. Create a new plugin named `jax-demo` in `src/plugins/jax-demo`.
2. Use `pixi` to manage the Python environment and JAX dependencies.
3. Implement a Python script (`app/main.py`) that uses JAX to process a geospatial dataset (e.g., Haversine distance calculation on large arrays).
4. Demonstrate JAX features like `jit` (Just-In-Time compilation) and `vmap` (vectorization).
5. Drive all development using Go tests in `src/plugins/jax-demo/test/`.

## Non-Goals
1. DO NOT install Python system-wide; use `pixi` locally in the plugin directory.
2. DO NOT use complex real-world GIS libraries (like GDAL) if simple numpy/jax arrays suffice for demonstration.

## Test
1. All plugin tests are run with `./dialtone.sh plugin test jax-demo`.
2. Tests should verify that `pixi` is set up correctly and the JAX script runs successfully, producing the expected output.
3. Tests are written in Golang (e.g., `integration_test.go` executing the pixi commands).

## Subtask: Research
- description: Read "Thinking in JAX" to understand `numpy` vs `jax.numpy`, `jit`, and `vmap`.
- description: Design a simple geospatial problem: Calculating distances between a reference point and 1 million random points using the Haversine formula.
- status: todo

## Subtask: Scaffold
- description: Create plugin directory structure `src/plugins/jax-demo/{app,cli,test}` using `plugin create`.
- description: [NEW] `src/plugins/jax-demo/app/pixi.toml`: Initialize pixi project with `jax`, `jaxlib`, and `numpy`.
- description: [NEW] `src/plugins/jax-demo/test/integration_test.go`: Create a test that runs `pixi run start` and checks for success.
- status: todo

## Subtask: Implementation
- description: [NEW] `src/plugins/jax-demo/app/main.py`: Implement the JAX logic.
    - Generate fake lat/lon data (e.g., 1 million points).
    - Implement Haversine formula using `jax.numpy`.
    - Apply `jax.jit` to speed up the function.
    - Use `jax.block_until_ready()` for accurate timing if needed.
- description: [NEW] `src/plugins/jax-demo/app/pixi.toml`: Define a `start` task to run `python main.py`.
- status: todo

## Subtask: Verification
- description: Run `./dialtone.sh plugin test jax-demo`.
- description: Verify the output shows JAX execution time and results.
- status: todo

## Code Samples

### likely pixi.toml
```toml
[project]
name = "jax-demo"
version = "0.1.0"
platforms = ["linux-64"]

[dependencies]
python = "3.11.*"
jax = "*"
jaxlib = "*"
numpy = "*"

[tasks]
start = "python main.py"
```

### likely main.py structure
```python
import jax.numpy as jnp
from jax import jit, random
import time

def haversine(lat1, lon1, lat2, lon2):
    R = 6371.0  # Earth radius in km
    dlat = jnp.radians(lat2 - lat1)
    dlon = jnp.radians(lon2 - lon1)
    a = jnp.sin(dlat / 2)**2 + jnp.cos(jnp.radians(lat1)) * jnp.cos(jnp.radians(lat2)) * jnp.sin(dlon / 2)**2
    c = 2 * jnp.arctan2(jnp.sqrt(a), jnp.sqrt(1 - a))
    return R * c

# Jitted execution
fast_haversine = jit(haversine)

# Generate data
key = random.PRNGKey(0)
lat1, lon1 = 40.7128, -74.0060  # NYC
# Generate arrays of random points
# ...

# Run
start = time.time()
result = fast_haversine(lat1, lon1, lat2_array, lon2_array).block_until_ready()
end = time.time()
print(f"Calculated {result.shape[0]} distances in {end - start:.4f}s")
```

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`