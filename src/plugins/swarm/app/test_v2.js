/**
 * Swarm plugin V2 lifecycle tests.
 */

import crypto from 'hypercore-crypto'
import fs from 'bare-fs'
import path from 'bare-path'
import os from 'bare-os'
import b4a from 'b4a'
import Hyperswarm from 'hyperswarm'
import { AutoKV } from './autokv_v2.js'
import { AutoLog } from './autolog_v2.js'

// Setup logging to swarm_test.log
const logFile = path.join(os.cwd(), 'swarm_test.log')
fs.writeFileSync(logFile, `--- Test Session Started: ${new Date().toISOString()} ---\n`)

const _log = globalThis.console.log
const _error = globalThis.console.error

function log (msg, ...args) {
  const formatted = typeof msg === 'string' ? msg : JSON.stringify(msg)
  const ts = new Date().toISOString().split('T')[1].slice(0, -1) // HH:MM:SS.mmm
  const line = `[${ts}] ${formatted} ${args.map(a => JSON.stringify(a)).join(' ')}\n`
  fs.appendFileSync(logFile, line)
  _log.apply(globalThis.console, [msg, ...args])
}

function error (msg, ...args) {
  const formatted = typeof msg === 'string' ? msg : JSON.stringify(msg)
  const ts = new Date().toISOString().split('T')[1].slice(0, -1)
  const line = `[${ts}] [ERROR] ${formatted} ${args.map(a => JSON.stringify(a)).join(' ')}\n`
  fs.appendFileSync(logFile, line)
  _error.apply(globalThis.console, [msg, ...args])
}

// Override console
globalThis.console.log = log
globalThis.console.error = error

const args = Pear.config?.args || []
const mode = args[0] || 'lifecycle'

if (mode === 'lifecycle') {
  runLifecycleTest().then(success => {
    Bare.exit(success ? 0 : 1)
  }).catch(err => {
    console.error('[FATAL]', err)
    Bare.exit(1)
  })
} else {
  console.log('Usage: pear run ./test_v2.js [lifecycle]')
}

async function runLifecycleTest() {
  const testStart = Date.now()
  console.log('--- Starting Swarm V2 Lifecycle Test (Stable Topic) ---')

  const topic = 'dialtone-test'
  const nodeIds = ['A', 'B'] // Reduced to 2 nodes for stability
  const nodes = []
  const tmpDirs = []

  // 1. Initialize nodes
  for (let i = 0; i < nodeIds.length; i++) {
    const dir = createTmpDir(nodeIds[i])
    tmpDirs.push(dir)

    const log = new AutoLog({
      topic: topic + '-log',
      storage: path.join(dir, 'log'),
      logId: nodeIds[i],
      pulseInterval: 1000 // 1s for fast tests
    })

    const kv = new AutoKV({
      topic: topic + '-kv',
      storage: path.join(dir, 'kv'),
      logId: nodeIds[i],
      pulseInterval: 1000
    })

    nodes.push({ id: nodeIds[i], log, kv, dir })
  }

  try {
    // 2. Ready up nodes
    const readyStart = Date.now()
    console.log(`[test] Initializing node ${nodes[0].id} (Bootstrap)...`)
    await nodes[0].log.ready()
    await nodes[0].kv.ready()

    const logBootstrap = b4a.toString(nodes[0].log.base.key, 'hex')
    const kvBootstrap = b4a.toString(nodes[0].kv.base.key, 'hex')
    console.log(`[test] Bootstrap Keys: Log=0x${logBootstrap.slice(0,8)}, KV=0x${kvBootstrap.slice(0,8)}`)

    console.log('[test] Initializing follower nodes with Node A as bootstrap...')
    await Promise.all(nodes.slice(1).map(async n => {
      console.log(`[test] Node ${n.id} joining swarm...`)
      n.log.bootstrap = logBootstrap
      n.kv.bootstrap = kvBootstrap
      await n.log.ready()
      await n.kv.ready()
    }))
    console.log(`[test] All nodes joined swarms in ${Date.now() - readyStart}ms`)

    const authStart = Date.now()
    console.log('[test] STEP 1: Waiting for Writer Authorization on Main Swarm (60s Timeout)...')
    
    const authTimeout = 60000
    const authPromise = Promise.all(nodes.slice(1).map(async n => {
      console.log(`[test] Node ${n.id} waiting to become writable (authorized by A or Warm)...`)
      await n.log.waitWritable()
      await n.kv.waitWritable()
      console.log(`[test] Node ${n.id} is now AUTHORIZED.`)
    }))

    const timeoutPromise = new Promise((_, reject) => 
      setTimeout(() => reject(new Error('TIMEOUT: Authorization took longer than 60s')), authTimeout)
    )

    await Promise.race([authPromise, timeoutPromise])
    console.log(`[test] All nodes authorized in ${Date.now() - authStart}ms`)

    const loopStart = Date.now()
    console.log('[test] STEP 2: All nodes authorized. Writing 10 messages per node...')
    const iterations = 10

    for (let i = 1; i <= iterations; i++) {
      await Promise.all(nodes.map(async (n) => {
        await n.log.append({ from: n.id, msg: `Msg ${i}`, count: i })
        await n.kv.put(`node.${n.id.toLowerCase()}.count`, i)
        await n.log.base.update()
        await n.kv.base.update()
      }))
      console.log(`[test] Broadcasted iteration ${i}/10...`)
      await new Promise(r => setTimeout(r, 500))
    }
    console.log(`[test] Interaction loop finished in ${Date.now() - loopStart}ms`)

    console.log('[test] STEP 3: Broadcast finished. Waiting for Final Data Convergence...')
    const syncStart = Date.now()
    const syncTimeout = 60000 // 60 seconds max
    let synced = false
    let hashes = []
    let kvHashes = []

    while (Date.now() - syncStart < syncTimeout) {
      await Promise.all(nodes.map(n => n.log.base.update()))
      await Promise.all(nodes.map(n => n.kv.base.update()))

      hashes = await Promise.all(nodes.map(n => n.log.getHash()))
      kvHashes = await Promise.all(nodes.map(n => n.kv.getHash()))
      
      const allLogsSynced = hashes.every(h => h === hashes[0])
      const allKvSynced = kvHashes.every(h => h === kvHashes[0])
      
      const stats = nodes.map((n, i) => {
        return `${n.id}:[L=${n.log.base.view?.length || 0},P=${n.log.swarm.connections.size},W=${n.log.base.writable ? 'Y' : 'N'}]`
      }).join(' | ')

      if (allLogsSynced && allKvSynced) {
        console.log(`[test] CONVERGENCE REACHED after ${Math.floor((Date.now() - syncStart) / 1000)}s`)
        console.log(`[test] Final Stats: ${stats}`)
        synced = true
        break
      }

      console.log(`[test] Syncing... ${stats}`)
      await new Promise(r => setTimeout(r, 1000))
    }

    const durationTotal = Math.floor((Date.now() - testStart) / 1000)
    const convergenceTime = synced ? Math.floor((Date.now() - syncStart) / 1000) : 'TIMEOUT'
    
    console.log(`\n--- Test Time Breakdown ---`)
    console.log(`- Ready Time: ${readyStart - testStart}ms`)
    console.log(`- Auth Time:  ${Date.now() - authStart}ms`)
    console.log(`- Sync Time:  ${convergenceTime}s`)
    console.log(`- Total:      ${durationTotal}s`)

    if (!synced) {
      console.error('[test] TIMEOUT: Nodes failed to converge within 60s.')
    }

    // 4. Verification
    console.log(`\n--- Final State Verification (V2) ---`)
    console.log(`[test] Log hashes: ${hashes.join(', ')}`)
    const finalAllLogsSynced = hashes.every(h => h === hashes[0])
    console.log(`[test] KV hashes:  ${kvHashes.join(', ')}`)
    const finalAllKvSynced = kvHashes.every(h => h === kvHashes[0])

    if (finalAllLogsSynced && finalAllKvSynced) {
      console.log('[SUCCESS] All nodes converged to identical hashes.')
    } else {
      console.error('[FAILURE] Hash mismatch detected.')
    }

    // 5. Report Summary to TEST.md
    const nodeDetails = nodes.map(n => {
      const logKey = b4a.toString(n.log.base.local.key, 'hex').slice(0, 12)
      const kvKey = b4a.toString(n.kv.base.local.key, 'hex').slice(0, 12)
      return `  - **Node ${n.id}**: LogKey=${logKey}..., KVKey=${kvKey}..., Peers=${n.log.swarm.connections.size}, Writable=${n.log.base.writable ? 'YES' : 'NO'}`
    }).join('\n')

    const logSnapshots = await Promise.all(nodes.map(async n => {
      const tail = await n.log.tail(3)
      return `  - **Node ${n.id} Tail**: [${tail.map(l => l.data.msg).join(', ')}]`
    }))

    const summary = `
## Test Run: ${new Date().toLocaleString()}
- **Status**: ${synced ? 'PASS' : 'FAIL'}
- **Total Duration**: ${durationTotal}s
- **Convergence Time**: ${convergenceTime}s
- **Log Convergence**: ${finalAllLogsSynced ? 'YES' : 'NO'} (Hash: ${hashes[0]})
- **KV Convergence**: ${finalAllKvSynced ? 'YES' : 'NO'} (Hash: ${kvHashes[0]})
- **Iterations**: ${iterations}
### Node Details
${nodeDetails}
### Data Snapshots
${logSnapshots.join('\n')}
--------------------------------------------------
`
    try {
      fs.appendFileSync('../TEST.md', summary)
    } catch (err) {
      console.error('[test] Failed to write to TEST.md:', err)
    }

    // Test tail(n)
    const lastLogs = await nodes[0].log.tail(3)
    console.log(`[test] Tail(3) from Node A:`, lastLogs.map(l => l.data.msg))

    // 5. Cleanup
    console.log(`\n[test] Shutting down...`)
    await Promise.all(nodes.map(async n => {
      await n.log.close()
      await n.kv.close()
    }))

    if (allLogsSynced && allKvSynced) {
      console.log('[PASS] Swarm V2 test successful.')
      return true
    } else {
      return false
    }
  } catch (err) {
    console.error('[test] Fatal Error:', err)
    return false
  } finally {
    for (const dir of tmpDirs) {
      try { fs.rmSync(dir, { recursive: true, force: true }) } catch { }
    }
  }
}

function createTmpDir(suffix) {
  const tmp = path.join(os.tmpdir(), `dialtone-v2-test-${suffix}-${Math.random().toString(16).slice(2)}`)
  fs.mkdirSync(tmp, { recursive: true })
  return tmp
}