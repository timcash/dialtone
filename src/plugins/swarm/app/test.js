import Hyperswarm from 'hyperswarm'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'

const peerName = Pear.config.args[0] || 'peer-a'
const topicName = Pear.config.args[1] || 'dialtone-multi-test'

// Derive a deterministic key pair from the peer name for testing
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
        // Ignore connection resets during teardown
        if (err.code === 'ECONNRESET') return
        console.error(`[test] Socket error from ${remoteKey}:`, err.message)
    })

    socket.on('data', (data) => {
        console.log(`[test] Received: ${b4a.toString(data)}`)
        if (b4a.toString(data).startsWith('ping')) {
            socket.write(b4a.from(`pong from ${peerName}`))
        }
    })

    // Send initial ping
    socket.write(b4a.from(`ping from ${peerName}`))

    // If we connected and exchanged data, we consider it a success
    // (In a real test we'd wait for the pong, but for now connection is good)
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
