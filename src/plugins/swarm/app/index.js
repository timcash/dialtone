import Hyperswarm from 'hyperswarm'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'

const swarm = new Hyperswarm()
const topicName = Pear.config.args[0] || 'dialtone-default'
const topic = crypto.hash(b4a.from(topicName, 'utf8'))

console.log(`[swarm] Joining topic: ${topicName}`)
console.log(`[swarm] Topic hash: ${b4a.toString(topic, 'hex')}`)

swarm.on('connection', (socket, info) => {
  const peerKey = b4a.toString(info.publicKey, 'hex')
  console.log(`[swarm] Connected to peer: ${peerKey}`)

  socket.on('data', (data) => {
    console.log(`[swarm] Received data from ${peerKey}:`, b4a.toString(data))
  })

  socket.write(b4a.from(`Hello from ${Pear.config.name || 'unknown'}`))
})

const discovery = swarm.join(topic, { server: true, client: true })
discovery.flushed().then(() => {
  console.log(`[swarm] Swarm joined and flushed for topic: ${topicName}`)
})

Pear.teardown(async () => {
  console.log('[swarm] Shutting down...')
  await swarm.destroy()
})
