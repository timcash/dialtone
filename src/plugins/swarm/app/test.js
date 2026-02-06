/**
 * Swarm plugin tests (holepunch stack: hyperswarm, autobase, hyperbee).
 * Run with Pear:
 *   pear run ./test.js peer-a test-topic   — multi-peer hyperswarm test
 *   pear run ./test.js kv                  — Autobee K/V (autobase + hyperbee) test
 */

import Hyperswarm from 'hyperswarm'
import Autobase from 'autobase'
import Hyperbee from 'hyperbee'
import Hypercore from 'hypercore'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'
import fs from 'bare-fs'
import path from 'bare-path'
import os from 'bare-os'

const args = Pear.config?.args || []
const mode = args[0]

if (mode === 'kv') {
  runKvTest().catch((err) => {
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
  function createTmpDir () {
    const tmp = path.join(os.tmpdir(), 'dialtone-kv-' + Math.random().toString(16).slice(2))
    fs.mkdirSync(tmp, { recursive: true })
    return tmp
  }

  class Autobee {
    constructor (localCore, inputs) {
      this.base = new Autobase({
        inputs,
        localInput: localCore
      })
      this.bee = new Hyperbee(this.base.view, {
        extension: false,
        keyEncoding: 'utf-8',
        valueEncoding: 'json'
      })
    }

    async put (key, value) {
      return this.bee.put(key, value)
    }

    async get (key) {
      return this.bee.get(key)
    }

    async ready () {
      await this.base.ready()
      await this.bee.ready()
    }
  }

  console.log('--- Starting Swarm K/V (Autobee) Test [Ephemeral Mode] ---')

  const dirA = createTmpDir()
  const coreA = new Hypercore(dirA)
  await coreA.ready()
  console.log(`Peer A: ${b4a.toString(coreA.key, 'hex').slice(0, 8)}... `)

  const dirB = createTmpDir()
  const coreB = new Hypercore(dirB)
  await coreB.ready()
  console.log(`Peer B: ${b4a.toString(coreB.key, 'hex').slice(0, 8)}... `)

  const inputs = [coreA, coreB]
  const dbA = new Autobee(coreA, inputs)
  const dbB = new Autobee(coreB, inputs)

  await dbA.ready()
  await dbB.ready()
  console.log('Databases ready.')

  // Scenario 1: Sequential Write/Read
  console.log('\n[Scenario 1] Sequential Write')
  await dbA.put('status', 'online')
  console.log('Peer A wrote "status" = "online"')

  let s1 = dbA.base.replicate(true)
  let s2 = dbB.base.replicate(false)
  s1.pipe(s2).pipe(s1)
  await new Promise(r => setTimeout(r, 100))
  s1.destroy()
  s2.destroy()

  const ans = await dbB.get('status')
  console.log(`Peer B read "status": "${ans ? ans.value : 'null'}"`)

  if (ans && ans.value === 'online') {
    console.log('SUCCESS: Data synced from A to B')
  } else {
    console.error('FAILURE: Data did not sync')
    Bare.exit(1)
  }

  // Scenario 2: Concurrent Writes (Convergence)
  console.log('\n[Scenario 2] Concurrent Writes (Convergence)')
  const p1 = dbA.put('config.a', 1)
  const p2 = dbB.put('config.b', 2)
  await Promise.all([p1, p2])

  console.log('Syncing...')
  s1 = dbA.base.replicate(true)
  s2 = dbB.base.replicate(false)
  s1.pipe(s2).pipe(s1)
  await new Promise(r => setTimeout(r, 100))
  s1.destroy()
  s2.destroy()

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

  await coreA.close()
  await coreB.close()
  fs.rmSync(dirA, { recursive: true, force: true })
  fs.rmSync(dirB, { recursive: true, force: true })
  console.log('Cleaning up... Temporary directories removed.')
  Bare.exit(0)
}
