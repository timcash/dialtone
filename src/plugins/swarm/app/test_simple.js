import Hyperswarm from 'hyperswarm'
import Corestore from 'corestore'
import path from 'bare-path'
import os from 'bare-os'
import fs from 'bare-fs'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'

/**
 * Simple test for Hyperswarm + Corestore (No Autobase).
 * This verifies that two peers can find each other via a topic
 * and replicate a single Hypercore.
 */

async function main () {
  const topicName = 'dialtone-simple-test-' + Math.random().toString(16).slice(2, 8)
  const topic = crypto.hash(b4a.from(topicName, 'utf8'))
  const tmpDir = path.join(os.tmpdir(), 'dialtone-simple-' + Date.now())
  fs.mkdirSync(tmpDir, { recursive: true })

  console.log(`--- Starting Simple Swarm/Corestore Test ---`)
  console.log(`[test] Topic: ${topicName}`)
  console.log(`[test] Storage: ${tmpDir}`)

  // 1. Initialize Node A
  const storeA = new Corestore(path.join(tmpDir, 'node-a'))
  const swarmA = new Hyperswarm()
  const coreA = storeA.get({ name: 'main', valueEncoding: 'utf-8' })
  await coreA.ready()

  swarmA.on('connection', (socket) => {
    console.log('[Node A] Peer connected')
    storeA.replicate(socket)
  })
  const discoveryA = swarmA.join(topic, { server: true, client: true })

  // 2. Initialize Node B
  const storeB = new Corestore(path.join(tmpDir, 'node-b'))
  const swarmB = new Hyperswarm()
  
  swarmB.on('connection', (socket) => {
    console.log('[Node B] Peer connected')
    storeB.replicate(socket)
  })
  const discoveryB = swarmB.join(topic, { server: true, client: true })

  console.log('[test] Waiting for DHT discovery...')
  await Promise.all([discoveryA.flushed(), discoveryB.flushed()])

  // 3. Wait for actual connection
  console.log('[test] Waiting for peers to link...')
  const start = Date.now()
  while (swarmA.connections.size === 0) {
    if (Date.now() - start > 15000) throw new Error('Timeout waiting for peer connection')
    await new Promise(r => setTimeout(r, 500))
  }
  console.log('[test] Peers linked!')

  // 4. Node A Appends Data
  console.log('[Node A] Appending message...')
  await coreA.append('Hello from Node A!')

  // 5. Node B Accesses the same core (by key)
  const coreB = storeB.get({ key: coreA.key, valueEncoding: 'utf-8' })
  await coreB.ready()

  console.log('[Node B] Syncing data...')
  // update() ensures B is aware of the latest length of A's core
  await coreB.update()
  
  const received = await coreB.get(0)
  console.log(`[Node B] Received data: "${received}"`)

  // 6. Verification
  if (received === 'Hello from Node A!') {
    console.log('--- [PASS] Simple Test Successful ---')
  } else {
    console.error('--- [FAIL] Data Mismatch ---')
    process.exit(1)
  }

  // Cleanup
  console.log('[test] Cleaning up...')
  await Promise.all([
    swarmA.destroy(),
    swarmB.destroy(),
    storeA.close(),
    storeB.close()
  ])
  try { fs.rmSync(tmpDir, { recursive: true, force: true }) } catch {}
  Bare.exit(0)
}

main().catch(err => {
  console.error('[FATAL]', err)
  Bare.exit(1)
})
