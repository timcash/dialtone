# Swarm Plugin Workflow (Incremental Refactor)

Use this workflow to develop and test the V2 architecture by moving through complexity levels.

## Folder Structures
Swarm plugin structure:
```shell
src/plugins/swarm/app/
├── autokv_v2.js              # V2 KV Class
├── autolog_v2.js             # V2 Log Class
├── test_level1_corestore.js  # PASS: Basic P2P Replication
├── test_level2_autobase.js   # STUCK: Manual Autobase Auth
├── test_level3_handshake.js  # PENDING: Auto-Auth Handshake
└── test_level4_full_v2.js    # PENDING: Integration Test
```

# Current Activity: Incremental Verification

We are systematically isolating why multi-writer authorization hangs.

## LEVEL 1: Corestore Replication (PASSED)
- **Goal**: Verify Node A can replicate a Hypercore to Node B via Hyperswarm.
- **Result**: Success. Hyperswarm and Corestore are functioning correctly.

## LEVEL 2: Static Autobase (CURRENT FOCUS)
- **Goal**: Node A authorizes Node B manually via `base.append({ addWriter: keyB })`.
- **Status**: **STUCK**. Authorization is not propagating to Node B within 10s.
- **Hypothesis**: Node A needs to be aware of Node B's local input core to correctly linearize and replicate.
- **Action**: Update Level 2 to ensure `store.replicate(socket)` is called on both sides and check for core discovery.

## LEVEL 3: Handshake Logic (PENDING)
- **Goal**: Automate Level 2 using a `:bootstrap` topic for key exchange.

## LEVEL 4: Full V2 Lifecycle (PENDING)
- **Goal**: Verify the production `AutoLog` and `AutoKV` classes using stable topics.

# Next Steps
1.  **Debug Level 2**: Refine the replication/auth flow in `test_level2_autobase_static.js`.
2.  **Verify Level 3**: Once static auth works, verify the automated handshake.
3.  **Refactor V2**: Apply working patterns from Level 2/3 back to `autolog_v2.js` and `autokv_v2.js`.
4.  **Finalize Integration**: Hook `test_v2.js` into `./dialtone.sh swarm test`.
