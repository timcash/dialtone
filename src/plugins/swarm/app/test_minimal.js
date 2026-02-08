import Hyperswarm from 'hyperswarm'
import Corestore from 'corestore'
import path from 'bare-path'
import os from 'bare-os'
import fs from 'bare-fs'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'

async function main () {
  const topicName = 'dialtone-minimal-test'
  const topic = crypto.hash(b4a.from(topicName, 'utf8'))
  const baseDir = path.join(os.tmpdir(), 'dialtone-minimal-' + Date.now())
  
  console.log(`--- Minimal Swarm/Corestore Test ---`)
  console.log(`Topic: ${topicName} (0x${b4a.toString(topic, 'hex').slice(0,8)})`)

  // 1. Setup Node A
  const storeA = new Corestore(path.join(baseDir, 'node-a'))
  const swarmA = new Hyperswarm({ mdns: true })
  const coreA = storeA.get({ name: 'main', valueEncoding: 'utf-8' })
  await coreA.ready()

  swarmA.on('connection', (socket) => {
    console.log('[A] Peer linked, starting replication...')
    storeA.replicate(socket)
  })
  swarmA.join(topic, { server: true, client: true })

  // 2. Setup Node B
  const storeB = new Corestore(path.join(baseDir, 'node-b'))
  const swarmB = new Hyperswarm({ mdns: true })
  
  swarmB.on('connection', (socket) => {
    console.log('[B] Peer linked, starting replication...')
    storeB.replicate(socket)
  })
  swarmB.join(topic, { server: true, client: true })

  console.log('Waiting for connection (max 30s)...')
  const start = Date.now()
  while (swarmA.connections.size === 0 || swarmB.connections.size === 0) {
    if (Date.now() - start > 30000) {
      console.error('[FAIL] Connection timeout. Check if another process is blocking ports.')
      await cleanup()
      Bare.exit(1)
    }
    await new Promise(r => setTimeout(r, 500))
  }

  console.log('Peers connected! Node A appending data...')
  await coreA.append('Minimal Test Message')

  console.log('Node B accessing core...')
  const coreB = storeB.get({ key: coreA.key, valueEncoding: 'utf-8' })
  await coreB.ready()
  
  console.log('Node B syncing...')
  await coreB.update()
  
  const result = await coreB.get(0)
  console.log(`Node B received: "${result}"`)

  if (result === 'Minimal Test Message') {
    console.log('--- [PASS] Minimal Test Successful ---')
  } else {
    console.error('--- [FAIL] Data mismatch ---')
  }

  async function cleanup () {
    console.log('Cleaning up...')
    await swarmA.destroy()
    await swarmB.destroy()
    await storeA.close()
    await storeB.close()
    try { fs.rmSync(baseDir, { recursive: true, force: true }) } catch {}
  }

  await cleanup()
  Bare.exit(0)
}

main().catch(err => {
  console.error('[FATAL]', err)
  Bare.exit(1)
})
