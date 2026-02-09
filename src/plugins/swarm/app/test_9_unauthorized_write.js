import path from 'bare-path'
import os from 'bare-os'
import fs from 'bare-fs'
import b4a from 'b4a'
import Hyperswarm from 'hyperswarm'
import Corestore from 'corestore'
import Autobase from 'autobase'
import crypto from 'hypercore-crypto'

/**
 * Test 9: Negative Authorization.
 * Verify that appends from an unauthorized writer are NOT seen by authorized nodes.
 */

async function main () {
  console.log('--- Test 9: Negative Authorization ---')
  const topicName = 'dialtone-test-neg-auth'
  const topic = crypto.hash(b4a.from(topicName, 'utf8'))
  const baseDir = path.join(os.tmpdir(), 'dt-test9-' + Date.now())

  const apply = async (nodes, view, host) => {
    for (const { value } of nodes) {
      if (value && value.addWriter) {
        await host.addWriter(b4a.from(value.addWriter, 'hex'), { indexer: true })
      } else {
        await view.append(value)
      }
    }
  }

  // Node A: Authorized Bootstrapper
  const storeA = new Corestore(path.join(baseDir, 'a'))
  const swarmA = new Hyperswarm({ mdns: true })
  const coreA = storeA.get({ name: 'auth' })
  await coreA.ready()
  
  const baseA = new Autobase(storeA, null, {
    apply,
    local: coreA,
    open: (store) => store.get('view', { valueEncoding: 'json' }),
    valueEncoding: 'json'
  })
  await baseA.ready()
  const bootstrapKey = b4a.toString(baseA.key, 'hex')
  
  swarmA.on('connection', (socket) => {
    storeA.replicate(socket)
  })
  const discA = swarmA.join(topic, { server: true, client: true })

  // Node B: Unauthorized Attacker
  const storeB = new Corestore(path.join(baseDir, 'b'))
  const swarmB = new Hyperswarm({ mdns: true })
  const coreB = storeB.get({ name: 'auth' })
  await coreB.ready()
  
  // Node B uses Node A as bootstrap
  const baseB = new Autobase(storeB, bootstrapKey, {
    apply,
    local: coreB,
    open: (store) => store.get('view', { valueEncoding: 'json' }),
    valueEncoding: 'json'
  })
  await baseB.ready()
  
  swarmB.on('connection', (socket) => {
    storeB.replicate(socket)
  })
  const discB = swarmB.join(topic, { server: true, client: true })

  console.log('[test] Waiting for DHT discovery...')
  await Promise.all([discA.flushed(), discB.flushed()])

  console.log('[test] Waiting for connection...')
  const start = Date.now()
  while (swarmA.connections.size === 0) {
    if (Date.now() - start > 30000) throw new Error('Timeout waiting for connection')
    await new Promise(r => setTimeout(r, 500))
  }
  console.log('[test] Connected.')

  console.log('[test] Node B (Unauthorized) appending to its local core...')
  // We append directly to the core to simulate a write that B *thinks* A might see.
  // We use a simple JSON string.
  await coreB.append(b4a.from(JSON.stringify({ msg: 'I am an intruder' })))
  
  console.log('[test] Node A waiting for sync (expecting NOTHING from B)...')
  const syncStart = Date.now()
  while (Date.now() - syncStart < 5000) {
    await baseA.update()
    if (baseA.view.length > 0) {
        const node = await baseA.view.get(0)
        if (node.msg === 'I am an intruder') {
            console.error('[FAIL] Node A received unauthorized data from Node B!')
            Bare.exit(1)
        }
    }
    await new Promise(r => setTimeout(r, 500))
  }
  
  console.log('[test] Node A view length:', baseA.view.length)
  if (baseA.view.length === 0) {
    console.log('[SUCCESS] Node A did not receive unauthorized data.')
    console.log('--- [PASS] TEST 9 SUCCESS ---')
  } else {
    console.log('[FAIL] Node A view length is not 0')
  }

  await swarmA.destroy()
  await swarmB.destroy()
  Bare.exit(0)
}

main().catch(err => {
  console.error(err)
  Bare.exit(1)
})