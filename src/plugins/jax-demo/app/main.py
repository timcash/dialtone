"""JAX geospatial demo: Haversine distance with jit + vmap + benchmarks."""

import time

import jax.numpy as jnp
import numpy as np
from jax import jit, random, vmap
import jax


def haversine(lat1, lon1, lat2, lon2):
    """Compute the great-circle distance in km between two points.

    Args:
        lat1: Reference latitude in degrees.
        lon1: Reference longitude in degrees.
        lat2: Target latitude in degrees (scalar or array).
        lon2: Target longitude in degrees (scalar or array).
    """
    earth_radius_km = 6371.0
    dlat = jnp.radians(lat2 - lat1)
    dlon = jnp.radians(lon2 - lon1)
    a = (
        jnp.sin(dlat / 2) ** 2
        + jnp.cos(jnp.radians(lat1))
        * jnp.cos(jnp.radians(lat2))
        * jnp.sin(dlon / 2) ** 2
    )
    c = 2 * jnp.arctan2(jnp.sqrt(a), jnp.sqrt(1 - a))
    return earth_radius_km * c


"""Vectorize over target points and jit compile for speed."""
vectorized_haversine = vmap(haversine, in_axes=(None, None, 0, 0))
fast_haversine = jit(vectorized_haversine)


def main():
    """Generate random points and benchmark NumPy vs JAX (cold/warm)."""
    ref_lat, ref_lon = 40.7128, -74.0060
    sizes = [10_000, 100_000, 1_000_000, 5_000_000]
    backend = jax.default_backend()
    backend_label = "CUDA" if backend == "gpu" else backend.upper()

    def haversine_np(lat1, lon1, lat2, lon2):
        earth_radius_km = 6371.0
        dlat = np.radians(lat2 - lat1)
        dlon = np.radians(lon2 - lon1)
        a = (
            np.sin(dlat / 2) ** 2
            + np.cos(np.radians(lat1))
            * np.cos(np.radians(lat2))
            * np.sin(dlon / 2) ** 2
        )
        c = 2 * np.arctan2(np.sqrt(a), np.sqrt(1 - a))
        return earth_radius_km * c

    print("--- Geospatial Benchmark ---")
    print("All timings are in milliseconds (ms).")
    print(f"Backend for JAX: {backend_label}")
    print(f"{'Size':>12} | {'Cold':>10} | {'Warm':>10} | {'NumPy':>10} | {'xFaster':>8}")
    print("-" * 66)

    for num_points in sizes:
        key = random.PRNGKey(0)
        lat_key, lon_key = random.split(key, 2)
        lat2 = random.uniform(lat_key, (num_points,), minval=-90.0, maxval=90.0)
        lon2 = random.uniform(lon_key, (num_points,), minval=-180.0, maxval=180.0)

        lat2_np = np.array(lat2)
        lon2_np = np.array(lon2)

        start = time.time()
        _ = haversine_np(ref_lat, ref_lon, lat2_np, lon2_np)
        numpy_ms = (time.time() - start) * 1000.0

        start = time.time()
        distances = fast_haversine(ref_lat, ref_lon, lat2, lon2)
        distances.block_until_ready()
        cold_ms = (time.time() - start) * 1000.0

        start = time.time()
        distances = fast_haversine(ref_lat, ref_lon, lat2, lon2)
        distances.block_until_ready()
        warm_ms = (time.time() - start) * 1000.0

        xfaster = numpy_ms / warm_ms if warm_ms > 0 else float("inf")
        print(
            f"{num_points:12,d} | {cold_ms:10.2f} | {warm_ms:10.2f} | "
            f"{numpy_ms:10.2f} | {xfaster:8.2f}"
        )

    print("-" * 66)
    print(f"Calculated {sizes[-1]} distances")
    print(f"Cold start took: {cold_ms/1000.0:.4f}s")
    print(f"Warm start took: {warm_ms/1000.0:.4f}s")
    print(f"Sample distance (km): {float(distances[0]):.2f}")


if __name__ == "__main__":
    main()
