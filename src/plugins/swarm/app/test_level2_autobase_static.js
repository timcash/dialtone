import Hyperswarm from 'hyperswarm'
import Corestore from 'corestore'
import Autobase from 'autobase'
import path from 'bare-path'
import os from 'bare-os'
import fs from 'bare-fs'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'

/**
 * Level 2: Static Autobase Authorization.
 * Verifies Node A can authorize Node B manually and they sync data.
 */

async function main () {
  const topic = crypto.hash(b4a.from('level2-test-' + Date.now(), 'utf8'))
  const baseDir = path.join(os.tmpdir(), 'dt-level2-' + Date.now())
  
  console.log('--- Level 2: Static Autobase ---')

  const apply = async (nodes, view, host) => {
    for (const { value } of nodes) {
      if (value && value.addWriter) {
        await host.addWriter(b4a.from(value.addWriter, 'hex'), { indexer: true })
      } else {
        await view.append(value)
      }
    }
  }

  // 1. Node A (Bootstrap)
  const storeA = new Corestore(path.join(baseDir, 'a'))
  const swarmA = new Hyperswarm({ mdns: true })
  
  const baseA = new Autobase(storeA, null, {
    apply,
    open: (store) => store.get('view', { valueEncoding: 'json' }),
    valueEncoding: 'json',
    ackInterval: 100
  })
  await baseA.ready()
  const bootstrapKey = baseA.key

  swarmA.on('connection', (socket) => storeA.replicate(socket))
  swarmA.join(topic, { server: true, client: true })

  // 2. Node B (Follower)
  const storeB = new Corestore(path.join(baseDir, 'b'))
  const swarmB = new Hyperswarm({ mdns: true })
  const baseB = new Autobase(storeB, bootstrapKey, {
    apply,
    open: (store) => store.get('view', { valueEncoding: 'json' }),
    valueEncoding: 'json',
    ackInterval: 100
  })
  await baseB.ready()

  swarmB.on('connection', (socket) => storeB.replicate(socket))
  swarmB.join(topic, { server: true, client: true })

  console.log('[test] Waiting for connection...')
  while (swarmA.connections.size === 0) await new Promise(r => setTimeout(r, 500))

  // 3. Authorization (A adds B)
  const keyB = b4a.toString(baseB.local.key, 'hex')
  console.log(`[test] Authorizing Node B (0x${keyB.slice(0,8)})...`)
  await baseA.append({ addWriter: keyB })
  await baseA.update()

  console.log('[test] Node B waiting to become writable...')
  const start = Date.now()
  while (!baseB.writable) {
    if (Date.now() - start > 10000) throw new Error('Timeout waiting for auth')
    await baseB.update()
    await new Promise(r => setTimeout(r, 500))
  }
  console.log('[test] Node B is writable!')

  await baseB.append({ msg: 'Level 2 Success' })
  
  console.log('[test] Finalizing sync...')
  await baseB.ack()
  await baseA.update()
  await baseA.ack()
  await baseB.update()
  await baseA.update()

  console.log(`[test] Node A view length: ${baseA.view.length}`)
  let result = null
  for (let i = 0; i < baseA.view.length; i++) {
    const node = await baseA.view.get(i)
    console.log(`[test] Node A view[${i}]:`, node)
    if (node?.msg === 'Level 2 Success') result = node
  }

  if (result) {
    console.log(`[test] Node A received: ${result.msg}`)
    console.log('--- [PASS] LEVEL 2 SUCCESS ---')
  } else {
    console.error('--- [FAIL] LEVEL 2 MISMATCH ---')
  }

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