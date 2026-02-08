import Hyperswarm from 'hyperswarm'
import Corestore from 'corestore'
import Autobase from 'autobase'
import path from 'bare-path'
import os from 'bare-os'
import fs from 'bare-fs'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'

/**
 * Minimal Autobase Test (2 Nodes).
 * Verifies that Node A can authorize Node B and they see the same linearized view.
 */

async function main () {
  const topicName = 'dialtone-autobase-minimal'
  const topic = crypto.hash(b4a.from(topicName, 'utf8'))
  const baseDir = path.join(os.tmpdir(), 'dialtone-autobase-' + Date.now())
  
  console.log(`--- Minimal Autobase Test ---`)

  // 1. Setup Node A (The Bootstrap)
  const storeA = new Corestore(path.join(baseDir, 'node-a'))
  const swarmA = new Hyperswarm({ mdns: true })
  
  // Get A's local key first to use as bootstrap
  const coreA = storeA.get({ name: 'autobase' })
  await coreA.ready()
  const bootstrapA = b4a.toString(coreA.key, 'hex')

  const baseA = new Autobase(storeA, bootstrapA, {
    apply: (nodes, view) => {
      for (const node of nodes) {
        if (node.value.addWriter) {
          view.append({ type: 'sys', msg: `Added writer ${node.value.addWriter.slice(0,8)}` })
        } else {
          view.append(node.value)
        }
      }
    },
    open: (store) => store.get('view', { valueEncoding: 'json' }),
    valueEncoding: 'json'
  })
  await baseA.ready()

  swarmA.on('connection', (socket) => {
    console.log('[A] Peer connected')
    storeA.replicate(socket)
  })
  swarmA.join(topic, { server: true, client: true })

  // 2. Setup Node B
  const storeB = new Corestore(path.join(baseDir, 'node-b'))
  const swarmB = new Hyperswarm({ mdns: true })
  const baseB = new Autobase(storeB, bootstrapA, { // B joins A's bootstrap
    apply: (nodes, view) => {
      for (const node of nodes) {
        if (node.value.addWriter) {
          view.append({ type: 'sys', msg: `Added writer ${node.value.addWriter.slice(0,8)}` })
        } else {
          view.append(node.value)
        }
      }
    },
    open: (store) => store.get('view', { valueEncoding: 'json' }),
    valueEncoding: 'json'
  })
  await baseB.ready()

  swarmB.on('connection', (socket) => {
    console.log('[B] Peer connected')
    storeB.replicate(socket)
  })
  swarmB.join(topic, { server: true, client: true })

  console.log('[test] Waiting for nodes to connect...')
  while (swarmA.connections.size === 0) await new Promise(r => setTimeout(r, 500))
  console.log('[test] Connected! Node A authorizing Node B...')

  // 3. Authorization Flow
  const keyB = b4a.toString(baseB.local.key, 'hex')
  await baseA.append({ addWriter: keyB })
  await baseA.update()

  console.log('[test] Node B waiting to become writable...')
  const authStart = Date.now()
  while (!baseB.writable) {
    if (Date.now() - authStart > 10000) throw new Error('Timeout waiting for B to become writable')
    await baseB.update()
    await new Promise(r => setTimeout(r, 500))
  }
  console.log('[test] Node B is now writable!')

  // 4. Data Sync Flow
  console.log('[test] Node B appending data...')
  await baseB.append({ from: 'B', msg: 'Hello from B!' })
  await baseB.update()

  console.log('[test] Node A syncing...')
  await baseA.update()
  
  const viewA = baseA.view
  const viewB = baseB.view
  
  console.log(`[test] Node A View Length: ${viewA.length}`)
  console.log(`[test] Node B View Length: ${viewB.length}`)

  if (viewA.length === viewB.length && viewA.length > 0) {
    const finalMsg = await viewA.get(viewA.length - 1)
    console.log(`[test] Node A last msg: "${finalMsg.msg}"`)
    console.log('--- [PASS] Minimal Autobase Test Successful ---')
  } else {
    console.error('--- [FAIL] View mismatch ---')
  }

  // Cleanup
  await swarmA.destroy()
  await swarmB.destroy()
  await storeA.close()
  await storeB.close()
  Bare.exit(0)
}

main().catch(err => {
  console.error('[FATAL]', err)
  Bare.exit(1)
})
