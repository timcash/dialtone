/**
 * Swarm plugin tests (holepunch stack: hyperswarm, autobase, hyperbee).
 * Run with Pear:
 *   pear run ./test.js peer-a test-topic   — multi-peer hyperswarm test
 *   pear run ./test.js kv                  — Autobee K/V (autobase + hyperbee) test
 *   pear run ./test.js session             — Autobase session log test
 */

import b4a from 'b4a'
import crypto from 'hypercore-crypto'
import fs from 'bare-fs'
import path from 'bare-path'
import os from 'bare-os'
import { AutoKV } from './autokv.js'
import { AutoLog } from './autolog.js'

const args = Pear.config?.args || []
const mode = args[0]

if (mode === 'kv') {
  runKvTest().catch((err) => {
    console.error(err)
    Bare.exit(1)
  })
} else if (mode === 'session') {
  runSessionTest().catch((err) => {
    console.error(err)
    Bare.exit(1)
  })
} else {
  runMultiPeerTest()
}

// ---------------------------------------------------------------------------
// Multi-peer Hyperswarm test (existing)
// ---------------------------------------------------------------------------
function runMultiPeerTest () {
  const peerName = args[0] || 'peer-a'
  const topicName = args[1] || 'dialtone-multi-test'

  const seed = crypto.hash(b4a.from(peerName, 'utf8'))
  const keyPair = crypto.keyPair(seed)
  const swarm = new Hyperswarm({ keyPair })
  const topic = crypto.hash(b4a.from(topicName, 'utf8'))

  console.log(`[test] Peer: ${peerName} (Public Key: ${b4a.toString(keyPair.publicKey, 'hex')})`)
  console.log(`[test] Joining topic: ${topicName}`)

  swarm.on('connection', (socket, info) => {
    const remoteKey = b4a.toString(info.publicKey, 'hex')
    console.log(`[test] Connected to peer: ${remoteKey}`)

    socket.on('error', (err) => {
      if (err.code === 'ECONNRESET') return
      console.error(`[test] Socket error from ${remoteKey}:`, err.message)
    })

    socket.on('data', (data) => {
      console.log(`[test] Received: ${b4a.toString(data)}`)
      if (b4a.toString(data).startsWith('ping')) {
        socket.write(b4a.from(`pong from ${peerName}`))
      }
    })

    socket.write(b4a.from(`ping from ${peerName}`))
    setTimeout(() => {
      console.log(`[test] ${peerName} test complete.`)
      Bare.exit(0)
    }, 2000)
  })

  swarm.join(topic)
  setTimeout(() => {
    console.error(`[test] ${peerName} timed out after 10s`)
    Bare.exit(1)
  }, 10000)
}

// ---------------------------------------------------------------------------
// Autobee K/V test (autobase + hyperbee) — migrated from test/kv.ts
// ---------------------------------------------------------------------------
async function runKvTest () {
  // K/V test exercises Autobase + Hyperbee convergence for a single topic.
  console.log('--- Starting Swarm K/V (Autobee) Test [Ephemeral Mode] ---')

  const agentIds = ['a', 'b', 'c']
  const agents = []
  const agentDirs = []

  // Create isolated stores per peer so each writer has its own local state.
  for (let i = 0; i < agentIds.length; i++) {
    const dir = createTmpDir()
    console.log(`DIALTONE> Created temp dir for KV agent ${agentIds[i].toUpperCase()}: ${dir}`)
    const kvDir = path.join(dir, 'kv')
    fs.mkdirSync(kvDir, { recursive: true })
    agentDirs.push(dir)
    agents.push(new AutoKV({
      topic: 'dialtone-kv',
      storage: kvDir,
      keepBootstrapHost: i === 0,
      requireBootstrap: i !== 0
    }))
  }

  // Start host first, then join others in parallel.
  console.log('DIALTONE> KV agent A ready() begin')
  await agents[0].ready()
  console.log('DIALTONE> KV agent A ready() complete')
  console.log(`Peer A: ${b4a.toString(agents[0].base.local.key, 'hex').slice(0, 8)}... `)

  await Promise.all(agents.slice(1).map(async (agent, idx) => {
    const i = idx + 1
    console.log(`DIALTONE> KV agent ${agentIds[i].toUpperCase()} ready() begin`)
    await agent.ready()
    console.log(`DIALTONE> KV agent ${agentIds[i].toUpperCase()} ready() complete`)
    console.log(`Peer ${agentIds[i].toUpperCase()}: ${b4a.toString(agent.base.local.key, 'hex').slice(0, 8)}... `)
  }))

  console.log('Databases ready.')

  // Scenario 1: Sequential write on A, read from B after replication.
  console.log('\n[Scenario 1] Sequential Write')
  await agents[0].put('status', 'online')
  console.log('Peer A wrote "status" = "online"')

  await waitForKvReplication(agents, 1)

  const ans = await agents[1].get('status')
  console.log(`Peer B read "status": "${ans ? ans.value : 'null'}"`)

  if (ans && ans.value === 'online') {
    console.log('SUCCESS: Data synced from A to B')
  } else {
    console.error('FAILURE: Data did not sync')
    Bare.exit(1)
  }

  // Scenario 2: Concurrent writes on A/B/C should converge after replication.
  console.log('\n[Scenario 2] Concurrent Writes (Convergence)')
  const keys = ['config.a', 'config.b', 'config.c']
  const writes = keys.map((key, idx) => agents[idx].put(key, idx + 1))
  await Promise.all(writes)

  console.log('Syncing...')
  await waitForKvReplication(agents, 3)

  const valuesByAgent = []
  for (let i = 0; i < agents.length; i++) {
    const values = await Promise.all(keys.map((key) => agents[i].get(key)))
    const nums = values.map((v) => v?.value)
    valuesByAgent.push(nums)
    console.log(`Peer ${agentIds[i].toUpperCase()} Sees: a=${nums[0]}, b=${nums[1]}, c=${nums[2]}`)
  }

  const expected = [1, 2, 3]
  const ok = valuesByAgent.every((nums) => (
    nums.length === expected.length && nums.every((v, idx) => v === expected[idx])
  ))

  if (ok) {
    console.log('SUCCESS: All peers converged to the same state (Union of all writes)')
  } else {
    console.error('FAILURE: State did not converge correctly')
    Bare.exit(1)
  }

  await Promise.all(agents.map((agent) => agent.close()))
  agentDirs.forEach((dir) => {
    fs.rmSync(dir, { recursive: true, force: true })
  })
  // Cleanup temp stores to avoid leaking local state between runs.
  console.log('Cleaning up... Temporary directories removed.')
  Bare.exit(0)
}

// ---------------------------------------------------------------------------
// Session log test (autobase + corestore)
// ---------------------------------------------------------------------------
async function runSessionTest () {
  // Session test exercises Autobase + Corestore log convergence on a different topic.
  console.log('DIALTONE> Starting Swarm Session Log Test [Ephemeral Mode]...')

  const agentIds = ['a', 'b', 'c', 'd', 'e']
  const agents = []
  const agentDirs = []

  // Separate stores per peer for the session log topic.
  for (let i = 0; i < agentIds.length; i++) {
    const dir = createTmpDir()
    console.log(`DIALTONE> Created temp dir for session agent ${agentIds[i].toUpperCase()}: ${dir}`)
    const sessionsDir = path.join(dir, 'sessions')
    fs.mkdirSync(sessionsDir, { recursive: true })
    agentDirs.push(dir)
    agents.push(new AutoLog({
      topic: 'dialtone-session-log',
      storage: sessionsDir,
      keepBootstrapHost: i === 0,
      requireBootstrap: i !== 0
    }))
  }

  // Start host first, then join others in parallel.
  console.log('DIALTONE> Session agent A ready() begin')
  await agents[0].ready()
  console.log('DIALTONE> Session agent A ready() complete')

  await Promise.all(agents.slice(1).map(async (agent, idx) => {
    const i = idx + 1
    console.log(`DIALTONE> Session agent ${agentIds[i].toUpperCase()} ready() begin`)
    await agent.ready()
    console.log(`DIALTONE> Session agent ${agentIds[i].toUpperCase()} ready() complete`)
  }))

  console.log('DIALTONE> Session logs ready. Running multi-writer append...')

  // Each peer appends a session event; replication should converge via swarm.
  await Promise.all(agents.map((agent, idx) => agent.append({ peer: agentIds[idx], action: 'join' })))
  await waitForSessionReplication(agents, agentIds.length)

  const eventsByAgent = []
  for (let i = 0; i < agents.length; i++) {
    const events = await agents[i].list()
    eventsByAgent.push(events)
    console.log(`LLM-TEST> Peer ${agentIds[i].toUpperCase()} Sessions: ${events.map(s => `${s.peer}:${s.action}`).join(', ')}`)
  }

  const expected = new Set(agentIds.map((id) => `${id}:join`))
  const allOk = eventsByAgent.every((events) => (
    events.every(s => expected.has(`${s.peer}:${s.action}`)) && events.length === agentIds.length
  ))

  const logs = agents.map((agent) => getSessionLog(agent.base))
  const lengths = logs.map((log) => log.length)
  const sameLength = lengths.every((len) => len === lengths[0])

  if (allOk && sameLength) {
    console.log('LLM-TEST> [SUCCESS] Session logs merged via Autobase')
  } else {
    console.error('LLM-TEST> [ERROR] Session logs did not converge correctly')
    Bare.exit(1)
  }

  await Promise.all(agents.map((agent) => agent.close()))
  agentDirs.forEach((dir) => {
    fs.rmSync(dir, { recursive: true, force: true })
  })
  // Cleanup temp stores to avoid leaking local state between runs.
  console.log('DIALTONE> Cleaning up... Temporary directories removed.')
  Bare.exit(0)
}

function createTmpDir () {
  const tmp = path.join(os.tmpdir(), 'dialtone-kv-' + Math.random().toString(16).slice(2))
  fs.mkdirSync(tmp, { recursive: true })
  return tmp
}


async function waitForSessionReplication (sessions, expectedCount) {
  const deadline = Date.now() + 10000
  while (Date.now() < deadline) {
    const ready = await Promise.all(sessions.map(async (s) => {
      const log = getSessionLog(s.base)
      return log.length >= expectedCount
    }))
    if (ready.every(Boolean)) {
      await Promise.all(sessions.map((s) => s.base.update()))
      return
    }
    await new Promise(r => setTimeout(r, 200))
  }
  throw new Error('Timed out waiting for session log replication over swarm')
}

function getSessionLog (base) {
  return base.view || base._viewStore.get('autolog', { valueEncoding: 'json' })
}

async function waitForKvReplication (peers, expectedVersion) {
  const deadline = Date.now() + 10000
  while (Date.now() < deadline) {
    const versions = peers.map((p) => p.bee.version || 0)
    const lengths = peers.map((p) => p.bee.core.length || 0)
    const minVersion = Math.min(...versions)
    const allEqual = versions.every((v) => v === versions[0]) && lengths.every((l) => l === lengths[0])
    if (minVersion >= expectedVersion && allEqual) {
      return
    }
    await new Promise(r => setTimeout(r, 200))
  }
  throw new Error('Timed out waiting for K/V replication over swarm')
}
