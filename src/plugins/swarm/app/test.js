/**
 * Swarm plugin lifecycle tests.
 * This test demonstrates how to use AutoLog and AutoKV for decentralized collaboration.
 * 
 * Run with:
 *   ./dialtone.sh swarm test
 */

import crypto from 'hypercore-crypto'
import fs from 'bare-fs'
import path from 'bare-path'
import os from 'bare-os'
import b4a from 'b4a'
import Hyperswarm from 'hyperswarm'
import { AutoKV } from './autokv.js'
import { AutoLog } from './autolog.js'

const args = Pear.config?.args || []
const mode = args[0] || 'lifecycle'

if (mode === 'lifecycle') {
  runLifecycleTest().then(success => {
    Bare.exit(success ? 0 : 1)
  }).catch(err => {
    console.error('[FATAL]', err)
    Bare.exit(1)
  })
} else if (mode === 'kv') {
  runKvTest().catch(err => {
    console.error('[FATAL]', err)
    Bare.exit(1)
  })
} else {
  console.log('Usage: pear run ./test.js [lifecycle|kv]')
}

/**
 * Main lifecycle test:
 * - Spawns 3 nodes (A, B, C)
 * - Each node runs in a loop for 30 seconds
 * - Nodes periodically append to a shared log and update a shared KV store
 * - After the loop, we verify all nodes have converged to the same state
 */
async function runLifecycleTest() {
  console.log('--- Starting Swarm Lifecycle Test (30s) ---')

  const topic = 'dt-' + Math.random().toString(16).slice(2, 8)
  const nodeIds = ['A', 'B', 'C']
  const nodes = []
  const tmpDirs = []

  // 1. Initialize nodes
  for (let i = 0; i < nodeIds.length; i++) {
    const dir = createTmpDir(nodeIds[i])
    tmpDirs.push(dir)

    const keySwarm = new Hyperswarm()

    const log = new AutoLog({
      topic: topic + '-log',
      storage: path.join(dir, 'log'),
      swarm: new Hyperswarm(),
      keySwarm: keySwarm,
      requireBootstrap: i !== 0,
      logId: nodeIds[i]
    })

    const kv = new AutoKV({
      topic: topic + '-kv',
      storage: path.join(dir, 'kv'),
      swarm: new Hyperswarm(),
      keySwarm: keySwarm,
      requireBootstrap: i !== 0,
      logId: nodeIds[i]
    })

    nodes.push({ id: nodeIds[i], log, kv, dir, keySwarm })
  }

  try {
    // 2. Ready up nodes (Bootstrap first)
    console.log(`[test] Initializing node ${nodes[0].id} (Bootstrap)...`)
    await nodes[0].log.ready()
    await nodes[0].kv.ready()
    console.log(`[test] Node A Writer Keys: Log=${b4a.toString(nodes[0].log.base.local.key, 'hex')}, KV=${b4a.toString(nodes[0].kv.base.local.key, 'hex')}`)

    console.log('[test] Initializing follower nodes...')
    await Promise.all(nodes.slice(1).map(async n => {
      await n.log.ready()
      await n.kv.ready()
      console.log(`[test] Node ${n.id} Writer Keys: Log=${b4a.toString(n.log.base.local.key, 'hex')}, KV=${b4a.toString(n.kv.base.local.key, 'hex')}`)
    }))

    console.log('[test] Waiting for follower nodes to be authorized as writers...')
    await Promise.all(nodes.slice(1).map(async n => {
      await n.log.waitWritable()
      await n.kv.waitWritable()
    }))

    console.log('[test] All nodes ready and authorized. Starting 30s interaction loop...')

    // 3. Interaction loop
    const duration = 30000
    const start = Date.now()
    let iterations = 0

    while (Date.now() - start < duration) {
      iterations++
      const elapsed = Math.floor((Date.now() - start) / 1000)

      // Each node acts randomly
      await Promise.all(nodes.map(async (n) => {
        const action = Math.random()
        if (action < 0.2) {
          await n.log.append({ from: n.id, msg: `Hello at ${elapsed}s`, iter: iterations })
        } else if (action < 0.4) {
          await n.kv.put(`status.${n.id.toLowerCase()}`, { online: true, lastSeen: elapsed })
        }
      }))

      if (iterations % 5 === 0) {
        console.log(`[test] ${elapsed}s elapsed... Nodes: ${nodes.map(n => n.id).join(', ')}`)
      }

      await new Promise(r => setTimeout(r, 1000))
    }

    console.log('[test] Loop finished. Waiting 10s for final synchronization...')
    await new Promise(r => setTimeout(r, 10000))

    // 4. Verification
    console.log('\n--- Final State Verification ---')

    const finalLogs = await Promise.all(nodes.map(n => n.log.list()))
    const logLengths = finalLogs.map(l => l.length)
    console.log(`[test] Log lengths: ${logLengths.join(', ')}`)

    const allLogsSame = logLengths.every(l => l === logLengths[0] && l > 0)
    if (allLogsSame) {
      console.log('[SUCCESS] All nodes converged to the same log length.')
    } else {
      console.error('[FAILURE] Log length mismatch or no entries.')
    }

    const kvStates = []
    for (const n of nodes) {
      const states = {}
      for (const id of nodeIds) {
        const key = `status.${id.toLowerCase()}`
        const val = await n.kv.get(key)
        states[key] = val?.value
      }
      kvStates.push(states)
    }

    console.log('[test] KV states (Node A):', kvStates[0])

    const allKvSame = kvStates.every(s => JSON.stringify(s) === JSON.stringify(kvStates[0]))
    if (allKvSame) {
      console.log('[SUCCESS] All nodes converged to the same KV state.')
    } else {
      console.error('[FAILURE] KV state mismatch.')
      for (let i = 0; i < nodes.length; i++) {
        console.log(`Node ${nodes[i].id}:`, JSON.stringify(kvStates[i]))
      }
    }

    // 5. Cleanup
    console.log('\n[test] Shutting down nodes...')
    await Promise.all(nodes.map(async n => {
      await n.log.close()
      await n.kv.close()
    }))

    if (allLogsSame && allKvSame) {
      console.log('[PASS] Full lifecycle test successful.')
      return true
    } else {
      console.log('[FAIL] Convergence failed.')
      return false
    }
  } catch (err) {
    console.error('[test] Fatal Error during lifecycle test:', err)
    return false
  } finally {
    console.log('[test] Cleaning up temporary directories...')
    for (const dir of tmpDirs) {
      try {
        fs.rmSync(dir, { recursive: true, force: true })
      } catch { }
    }
  }
}

async function runKvTest() {
  console.log('--- Starting Simple KV Test ---')
  const dir = createTmpDir('kv-simple')
  const kv = new AutoKV({
    topic: 'simple-kv-' + Math.random().toString(16).slice(2),
    storage: path.join(dir, 'kv')
  })

  await kv.ready()
  await kv.put('hello', 'world')
  const val = await kv.get('hello')
  console.log(`Read 'hello': ${val?.value}`)

  await kv.close()
  fs.rmSync(dir, { recursive: true, force: true })

  if (val?.value === 'world') {
    console.log('[PASS] Simple KV test successful.')
  } else {
    console.log('[FAIL] Simple KV test failed.')
    Bare.exit(1)
  }
}

function createTmpDir(suffix) {
  const tmp = path.join(os.tmpdir(), `dialtone-test-${suffix}-${Math.random().toString(16).slice(2)}`)
  fs.mkdirSync(tmp, { recursive: true })
  return tmp
}
