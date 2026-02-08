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

  const apply = (nodes, view, host) => {
    for (const node of nodes) {
      if (node.value.addWriter) {
        host.addWriter(b4a.from(node.value.addWriter, 'hex'))
      } else {
        view.append(node.value)
      }
    }
  }

  // Node A (Bootstrap)
  const storeA = new Corestore(path.join(baseDir, 'a'))
  const swarmA = new Hyperswarm({ mdns: true })
  const keySwarmA = new Hyperswarm({ mdns: true })
  const coreA = storeA.get({ name: 'auth' })
  await coreA.ready()
  const bootstrapKey = b4a.toString(coreA.key, 'hex')

  const baseA = new Autobase(storeA, bootstrapKey, {
    apply,
    open: (store) => store.get('view', { valueEncoding: 'json' }),
    valueEncoding: 'json'
  })
  await baseA.ready()

  keySwarmA.on('connection', (socket) => {
    socket.write(`BASE_KEY:${bootstrapKey}
`)
    socket.on('data', (data) => {
      const line = data.toString()
      if (line.startsWith('WRITER_KEY:')) {
        const key = line.split(':')[1].trim()
        console.log(`[A] Handshake: Authorizing ${key.slice(0,8)}`)
        baseA.append({ addWriter: key }).then(() => baseA.update())
      }
    })
  })
  keySwarmA.join(keyTopic, { server: true, client: true })
  swarmA.on('connection', (socket) => storeA.replicate(socket))
  swarmA.join(topic, { server: true, client: true })

  // Node B (Follower)
  const storeB = new Corestore(path.join(baseDir, 'b'))
  const swarmB = new Hyperswarm({ mdns: true })
  const keySwarmB = new Hyperswarm({ mdns: true })
  let baseB = null

  keySwarmB.on('connection', (socket) => {
    const writerKey = b4a.toString(storeB.get({ name: 'auth' }).key, 'hex')
    socket.write(`WRITER_KEY:${writerKey}
`)
    socket.on('data', async (data) => {
      const line = data.toString()
      if (line.startsWith('BASE_KEY:') && !baseB) {
        const bKey = line.split(':')[1].trim()
        console.log(`[B] Handshake: Received BASE_KEY ${bKey.slice(0,8)}`)
        baseB = new Autobase(storeB, bKey, {
          apply,
          open: (store) => store.get('view', { valueEncoding: 'json' }),
          valueEncoding: 'json'
        })
        await baseB.ready()
      }
    })
  })
  keySwarmB.join(keyTopic, { server: true, client: true })
  swarmB.on('connection', (socket) => storeB.replicate(socket))
  swarmB.join(topic, { server: true, client: true })

  console.log('[test] Waiting for B to become writable...')
  const start = Date.now()
  while (!baseB || !baseB.writable) {
    if (Date.now() - start > 15000) throw new Error('Timeout waiting for handshake/auth')
    if (baseB) await baseB.update()
    await new Promise(r => setTimeout(r, 500))
  }
  console.log('[test] Node B is writable!')

  await baseB.append({ msg: 'Level 3 Success' })
  await baseB.update()
  await baseA.update()

  const result = await baseA.view.get(baseA.view.length - 1)
  console.log(`[test] Node A received: ${result.msg}`)

  if (result.msg === 'Level 3 Success') console.log('--- [PASS] LEVEL 3 SUCCESS ---')
  
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
