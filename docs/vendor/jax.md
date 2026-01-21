# JAX Documentation

## Overview
JAX is Autograd and XLA, brought together for high-performance machine learning research. It provides a system for writing standard Python/NumPy code that can be automatically differentiated and compiled to generic high-performance code (CPU, GPU, TPU).

## Usage in Dialtone
In the Dialtone environment, we use **Pixi** to manage Python environments and dependencies, including JAX.

### Prerequisites
- Ensure `pixi` is available (see [Pixi Documentation](pixi.md)).

### Setup
To use JAX in a plugin or script:

1.  **Initialize Pixi**:
    ```bash
    pixi init my_project
    cd my_project
    ```

2.  **Install JAX**:
    ```bash
    pixi add jax jaxlib numpy
    ```

3.  **Write Code**:
    Import `jax.numpy` as `jnp` and use it like standard NumPy, but with the added power of `jit`, `grad`, and `vmap`.

    ```python
    import jax.numpy as jnp
    from jax import grad, jit, vmap

    def predict(params, inputs):
        return jnp.dot(inputs, params)

    mse_jit = jit(grad(lambda params, inputs, targets: jnp.mean((predict(params, inputs) - targets)**2)))
    ```

4.  **Run**:
    ```bash
    pixi run python my_script.py
    ```

## Key Concepts
-   **Immutable Arrays**: Unlike NumPy, JAX arrays are immutable.
-   **Pure Functions**: JAX transformations (`jit`, `grad`, etc.) require pure functions (no side effects).
-   **XLA**: Accelerated Linear Algebra compiler that JAX uses under the hood.

## References
-   [JAX Documentation](https://jax.readthedocs.io/en/latest/)
-   [Thinking in JAX](https://jax.readthedocs.io/en/latest/notebooks/thinking_in_jax.html)
