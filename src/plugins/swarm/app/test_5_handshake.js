import Hyperswarm from 'hyperswarm'
import Corestore from 'corestore'
import Autobase from 'autobase'
import path from 'bare-path'
import os from 'bare-os'
import fs from 'bare-fs'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'

/**
 * Level 3: Handshake Logic.
 * Verifies that peers can exchange keys over a bootstrap topic and self-authorize.
 */

async function main () {
  const topicName = 'level3-test-' + Date.now()
  const topic = crypto.hash(b4a.from(topicName, 'utf8'))
  const keyTopic = crypto.hash(b4a.from(topicName + ':bootstrap', 'utf8'))
  const baseDir = path.join(os.tmpdir(), 'dt-level3-' + Date.now())
  
  console.log('--- Level 3: Automated Handshake ---')

  const apply = async (nodes, view, host) => {
    for (const { value } of nodes) {
      if (value && value.addWriter) {
        await host.addWriter(b4a.from(value.addWriter, 'hex'), { indexer: true })
      } else {
        await view.append(value)
      }
    }
  }

  // Node A (Bootstrap)
  const storeA = new Corestore(path.join(baseDir, 'a'))
  const swarmA = new Hyperswarm({ mdns: true })
  const keySwarmA = new Hyperswarm({ mdns: true })
  
  // Create 'auth' core manually to get stable key
  const coreA = storeA.get({ name: 'auth' })
  await coreA.ready()
  
  const baseA = new Autobase(storeA, null, {
    apply,
    open: (store) => store.get('view', { valueEncoding: 'json' }),
    valueEncoding: 'json',
    ackInterval: 100
  })
  await baseA.ready()
  const bootstrapKey = b4a.toString(baseA.key, 'hex')

  keySwarmA.on('connection', (socket) => {
    console.log('[A] KeySwarm connection!')
    socket.write(`BASE_KEY:${bootstrapKey}\n`)
    socket.on('data', (data) => {
      const line = data.toString()
      console.log(`[A] KeySwarm received: ${line.trim()}`)
      if (line.startsWith('WRITER_KEY:')) {
        const key = line.split(':')[1].trim()
        console.log(`[A] Handshake: Authorizing ${key.slice(0,8)}...`)
        baseA.append({ addWriter: key }).then(() => baseA.update())
      }
    })
  })
  const keyDiscAx = keySwarmA.join(keyTopic, { server: true, client: true })
  
  swarmA.on('connection', (socket) => storeA.replicate(socket))
  const discAx = swarmA.join(topic, { server: true, client: true })

  // Node B (Follower)
  const storeB = new Corestore(path.join(baseDir, 'b'))
  const swarmB = new Hyperswarm({ mdns: true })
  const keySwarmB = new Hyperswarm({ mdns: true })
  let baseB = null

  keySwarmB.on('connection', async (socket) => {
    console.log('[B] KeySwarm connection!')
    const coreB = storeB.get({ name: 'auth' })
    await coreB.ready()
    const writerKey = b4a.toString(coreB.key, 'hex')
    socket.write(`WRITER_KEY:${writerKey}\n`)
    socket.on('data', async (data) => {
      const line = data.toString()
      console.log(`[B] KeySwarm received: ${line.trim()}`)
      if (line.startsWith('BASE_KEY:') && !baseB) {
        const bKey = line.split(':')[1].trim()
        console.log(`[B] Handshake: Received BASE_KEY ${bKey.slice(0,8)}`)
        baseB = new Autobase(storeB, bKey, {
          apply,
          open: (store) => store.get('view', { valueEncoding: 'json' }),
          valueEncoding: 'json',
          ackInterval: 100
        })
        await baseB.ready()
        swarmB.on('connection', (socket) => storeB.replicate(socket))
        swarmB.join(topic, { server: true, client: true })
      }
    })
  })
  const keyDiscBx = keySwarmB.join(keyTopic, { server: true, client: true })

  console.log('[test] Waiting for DHT discovery...')
  await Promise.all([keyDiscAx.flushed(), keyDiscBx.flushed()])

  console.log('[test] Waiting for B to become writable...')
  const start = Date.now()
  while (!baseB || !baseB.writable) {
    if (Date.now() - start > 20000) throw new Error('Timeout waiting for handshake/auth')
    if (baseB) {
      await baseA.ack() // Node A acks to help B see the addWriter
      await baseB.update()
    }
    await new Promise(r => setTimeout(r, 1000))
  }
  console.log('[test] Node B is writable!')

  await baseB.append({ msg: 'Level 3 Success' })
  
  console.log('[test] Finalizing sync...')
  const syncStart = Date.now()
  let result = null
  while (Date.now() - syncStart < 10000) {
    await baseB.ack()
    await baseA.update()
    await baseA.ack()
    await baseB.update()
    await baseA.update()

    if (baseA.view.length > 0) {
      const node = await baseA.view.get(baseA.view.length - 1)
      if (node?.msg === 'Level 3 Success') {
        result = node
        break
      }
    }
    await new Promise(r => setTimeout(r, 1000))
  }

  if (result) {
    console.log(`[test] Node A received: ${result.msg}`)
    console.log('--- [PASS] LEVEL 3 SUCCESS ---')
  } else {
    console.error('--- [FAIL] LEVEL 3 SYNC TIMEOUT ---')
  }
  
  await swarmA.destroy()
  await swarmB.destroy()
  await keySwarmA.destroy()
  await keySwarmB.destroy()
  Bare.exit(0)
}

main().catch(err => {
    console.error(err)
    Bare.exit(1)
})
