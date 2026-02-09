import Hyperswarm from 'hyperswarm'
import Corestore from 'corestore'
import path from 'bare-path'
import os from 'bare-os'
import fs from 'bare-fs'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'

async function main () {
  const topic = crypto.hash(b4a.from('level1-test-' + Date.now(), 'utf8'))
  const baseDir = path.join(os.tmpdir(), 'dt-level1-' + Date.now())
  
  console.log('--- Level 1: Corestore Replication ---')

  const storeA = new Corestore(path.join(baseDir, 'a'))
  const swarmA = new Hyperswarm({ mdns: true })
  const coreA = storeA.get({ name: 'main', valueEncoding: 'utf-8' })
  await coreA.ready()

  swarmA.on('connection', (socket) => {
    console.log('[A] Connection!')
    storeA.replicate(socket)
  })
  const discoveryA = swarmA.join(topic, { server: true, client: true })

  const storeB = new Corestore(path.join(baseDir, 'b'))
  const swarmB = new Hyperswarm({ mdns: true })
  swarmB.on('connection', (socket) => {
    console.log('[B] Connection!')
    storeB.replicate(socket)
  })
  const discoveryB = swarmB.join(topic, { server: true, client: true })

  console.log('[test] Waiting for DHT discovery...')
  await Promise.all([discoveryA.flushed(), discoveryB.flushed()])

  console.log('[test] Waiting for connection...')
  const start = Date.now()
  while (swarmA.connections.size === 0) {
    if (Date.now() - start > 15000) {
        console.log('FAIL: Timeout waiting for connection')
        Bare.exit(1)
    }
    await new Promise(r => setTimeout(r, 500))
  }

  console.log('[test] Appending...')
  await coreA.append('Level 1 Success')
  
  const coreB = storeB.get({ key: coreA.key, valueEncoding: 'utf-8' })
  await coreB.ready()
  await coreB.update()
  
  const val = await coreB.get(0)
  console.log(`[test] Node B received: ${val}`)
  
  if (val === 'Level 1 Success') console.log('--- [PASS] LEVEL 1 SUCCESS ---')
  else console.log('--- [FAIL] LEVEL 1 DATA MISMATCH ---')

  await swarmA.destroy()
  await swarmB.destroy()
  await storeA.close()
  await storeB.close()
  try { fs.rmSync(baseDir, { recursive: true, force: true }) } catch {}
  Bare.exit(0)
}

main().catch(err => {
    console.error(err)
    Bare.exit(1)
})
