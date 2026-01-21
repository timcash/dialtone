import jax.numpy as jnp
from jax import jit, random, vmap
import time

def haversine(lat1, lon1, lat2, lon2):
    """
    Calculate the great circle distance between two points 
    on the earth (specified in decimal degrees)
    """
    R = 6371.0  # Earth radius in km
    
    # Convert decimal degrees to radians 
    dlat = jnp.radians(lat2 - lat1)
    dlon = jnp.radians(lon2 - lon1)
    
    # Haversine formula 
    a = jnp.sin(dlat / 2)**2 + jnp.cos(jnp.radians(lat1)) * jnp.cos(jnp.radians(lat2)) * jnp.sin(dlon / 2)**2
    c = 2 * jnp.arcsin(jnp.sqrt(a)) 
    return R * c

# Jitted version for speed
fast_haversine = jit(haversine)

# Vectorized version to handle arrays of points against a single point
# (Already handled by JNP broadcasting, but explicit vmap can be used)
# Actually, jnp.sin etc broadcast natively. 

import numpy as np

def benchmark():
    print("--- JAX vs NumPy Geospatial Benchmark ---")
    
    sizes = [10_000, 100_000, 1_000_000, 5_000_000]
    results = []

    # NYC Reference
    nyc_lat, nyc_lon = 40.7128, -74.0060

    print(f"{'Size':>12} | {'NumPy (s)':>12} | {'JAX Cold (s)':>12} | {'JAX Warm (s)':>12} | {'Speedup':>10}")
    print("-" * 70)

    for num_points in sizes:
        key = random.PRNGKey(42)
        keys = random.split(key, 2)
        
        # JAX Data
        j_lat2 = random.uniform(keys[0], (num_points,), minval=-90, maxval=90)
        j_lon2 = random.uniform(keys[1], (num_points,), minval=-180, maxval=180)
        
        # NumPy Data (converted from JAX for fairness in transfer if needed, or just generate new)
        n_lat2 = np.array(j_lat2)
        n_lon2 = np.array(j_lon2)

        # 1. NumPy Benchmark
        def haversine_np(lat1, lon1, lat2, lon2):
            R = 6371.0
            dlat = np.radians(lat2 - lat1)
            dlon = np.radians(lon2 - lon1)
            a = np.sin(dlat / 2)**2 + np.cos(np.radians(lat1)) * np.cos(np.radians(lat2)) * np.sin(dlon / 2)**2
            c = 2 * np.arcsin(np.sqrt(a))
            return R * c

        start = time.time()
        _ = haversine_np(nyc_lat, nyc_lon, n_lat2, n_lon2)
        numpy_time = time.time() - start

        # 2. JAX Cold Start
        start = time.time()
        # Note: We redfine/re-jit to force a "cold" start for each size if we want to show compilation overhead
        # or just use the same one. Let's use the same one to show it's already compiled for the formula 
        # but might need new tracing for the specific shape.
        _ = fast_haversine(nyc_lat, nyc_lon, j_lat2, j_lon2).block_until_ready()
        jax_cold_time = time.time() - start

        # 3. JAX Warm Start
        start = time.time()
        _ = fast_haversine(nyc_lat, nyc_lon, j_lat2, j_lon2).block_until_ready()
        jax_warm_time = time.time() - start

        speedup = numpy_time / jax_warm_time
        
        print(f"{num_points:12,d} | {numpy_time:12.4f} | {jax_cold_time:12.4f} | {jax_warm_time:12.4f} | {speedup:10.1f}x")
        results.append((num_points, numpy_time, jax_cold_time, jax_warm_time, speedup))

    print("-" * 70)
    print(f"Max Speedup: {max(r[4] for r in results):.1f}x")

if __name__ == "__main__":
    benchmark()
