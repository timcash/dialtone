/**
 * Swarm plugin tests (holepunch stack: hyperswarm, autobase, hyperbee).
 * Run with Pear:
 *   pear run ./test.js peer-a test-topic   — multi-peer hyperswarm test
 *   pear run ./test.js kv                  — Autobee K/V (autobase + hyperbee) test
 *   pear run ./test.js session             — Autobase session log test
 */

import Hyperswarm from 'hyperswarm'
import Corestore from 'corestore'
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

  // Create isolated stores per peer so each writer has its own local state.
  const dirA = createTmpDir()
  const kvDirA = path.join(dirA, 'kv')
  fs.mkdirSync(kvDirA, { recursive: true })

  const storeAkv = new Corestore(kvDirA)
  // Peer A: bootstrap the K/V Autobase instance.
  const dbA = new AutoKV(storeAkv, null)
  await dbA.ready()
  console.log(`Peer A: ${b4a.toString(dbA.base.local.key, 'hex').slice(0, 8)}... `)

  // Peer B: join Peer A's Autobase via bootstrap key.
  const dirB = createTmpDir()
  const kvDirB = path.join(dirB, 'kv')
  fs.mkdirSync(kvDirB, { recursive: true })

  const storeBkv = new Corestore(kvDirB)
  const dbB = new AutoKV(storeBkv, dbA.base.key)
  await dbB.ready()
  console.log(`Peer B: ${b4a.toString(dbB.base.local.key, 'hex').slice(0, 8)}... `)

  // Authorize Peer B as a writer, then replicate to sync metadata.
  await dbA.addWriter(dbB.base.local.key)
  await syncBases(dbA.base, dbB.base)
  console.log('Databases ready.')

  // Scenario 1: Sequential write on A, read from B after replication.
  console.log('\n[Scenario 1] Sequential Write')
  await dbA.put('status', 'online')
  console.log('Peer A wrote "status" = "online"')

  await syncBases(dbA.base, dbB.base)

  const ans = await dbB.get('status')
  console.log(`Peer B read "status": "${ans ? ans.value : 'null'}"`)

  if (ans && ans.value === 'online') {
    console.log('SUCCESS: Data synced from A to B')
  } else {
    console.error('FAILURE: Data did not sync')
    Bare.exit(1)
  }

  // Scenario 2: Concurrent writes on A/B should converge after replication.
  console.log('\n[Scenario 2] Concurrent Writes (Convergence)')
  const p1 = dbA.put('config.a', 1)
  const p2 = dbB.put('config.b', 2)
  await Promise.all([p1, p2])

  console.log('Syncing...')
  await syncBases(dbA.base, dbB.base)

  const valA1 = (await dbA.get('config.a'))?.value
  const valA2 = (await dbA.get('config.b'))?.value
  const valB1 = (await dbB.get('config.a'))?.value
  const valB2 = (await dbB.get('config.b'))?.value

  console.log(`Peer A Sees: a=${valA1}, b=${valA2}`)
  console.log(`Peer B Sees: a=${valB1}, b=${valB2}`)

  if (valA1 === 1 && valA2 === 2 && valB1 === 1 && valB2 === 2) {
    console.log('SUCCESS: Both peers converged to the same state (Union of all writes)')
  } else {
    console.error('FAILURE: State did not converge correctly')
    Bare.exit(1)
  }

  await storeAkv.close()
  await storeBkv.close()
  fs.rmSync(dirA, { recursive: true, force: true })
  fs.rmSync(dirB, { recursive: true, force: true })
  // Cleanup temp stores to avoid leaking local state between runs.
  console.log('Cleaning up... Temporary directories removed.')
  Bare.exit(0)
}

// ---------------------------------------------------------------------------
// Session log test (autobase + corestore)
// ---------------------------------------------------------------------------
async function runSessionTest () {
  // Session test exercises Autobase + Corestore log convergence on a different topic.
  console.log('--- Starting Swarm Session Log Test [Ephemeral Mode] ---')

  // Separate stores per peer for the session log topic.
  const dirA = createTmpDir()
  const sessionsDirA = path.join(dirA, 'sessions')
  fs.mkdirSync(sessionsDirA, { recursive: true })

  const storeAsessions = new Corestore(sessionsDirA)
  // Peer A: bootstrap the session log Autobase instance.
  const sessionsA = new AutoLog(storeAsessions, null)
  await sessionsA.ready()

  // Peer B: join Peer A's session Autobase via bootstrap key.
  const dirB = createTmpDir()
  const sessionsDirB = path.join(dirB, 'sessions')
  fs.mkdirSync(sessionsDirB, { recursive: true })

  const storeBsessions = new Corestore(sessionsDirB)
  const sessionsB = new AutoLog(storeBsessions, sessionsA.base.key)
  await sessionsB.ready()

  // Authorize Peer B as a session writer, then replicate to sync metadata.
  await sessionsA.addWriter(sessionsB.base.local.key)
  await syncBases(sessionsA.base, sessionsB.base)
  console.log('Session logs ready.')

  // Each peer appends a session event; replication should converge.
  await sessionsA.append({ peer: 'a', action: 'join' })
  await sessionsB.append({ peer: 'b', action: 'join' })
  await syncBases(sessionsA.base, sessionsB.base)

  const eventsA = await sessionsA.list()
  const eventsB = await sessionsB.list()
  console.log(`Peer A Sessions: ${eventsA.map(s => `${s.peer}:${s.action}`).join(', ')}`)
  console.log(`Peer B Sessions: ${eventsB.map(s => `${s.peer}:${s.action}`).join(', ')}`)
  const expected = new Set(['a:join', 'b:join'])
  const okA = eventsA.every(s => expected.has(`${s.peer}:${s.action}`)) && eventsA.length === 2
  const okB = eventsB.every(s => expected.has(`${s.peer}:${s.action}`)) && eventsB.length === 2
  if (okA && okB) {
    console.log('SUCCESS: Session logs merged via Autobase')
  } else {
    console.error('FAILURE: Session logs did not converge correctly')
    Bare.exit(1)
  }

  await storeAsessions.close()
  await storeBsessions.close()
  fs.rmSync(dirA, { recursive: true, force: true })
  fs.rmSync(dirB, { recursive: true, force: true })
  // Cleanup temp stores to avoid leaking local state between runs.
  console.log('Cleaning up... Temporary directories removed.')
  Bare.exit(0)
}

function createTmpDir () {
  const tmp = path.join(os.tmpdir(), 'dialtone-kv-' + Math.random().toString(16).slice(2))
  fs.mkdirSync(tmp, { recursive: true })
  return tmp
}


async function syncBases (baseA, baseB) {
  const s1 = baseA.replicate(true)
  const s2 = baseB.replicate(false)
  s1.pipe(s2).pipe(s1)
  await new Promise(r => setTimeout(r, 150))
  s1.destroy()
  s2.destroy()
  await baseA.update()
  await baseB.update()
}
