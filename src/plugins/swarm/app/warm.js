import { AutoLog } from './autolog.js'
import { AutoKV } from './autokv.js'
import path from 'bare-path'
import os from 'bare-os'
import fs from 'bare-fs'

async function main() {
    console.log('[warm] Bare.argv:', Bare.argv)
    const topicPrefix = Bare.argv[Bare.argv.length - 1].endsWith('.js') ? 'dialtone-warm' : Bare.argv[Bare.argv.length - 1]
    console.log(`--- Starting Dialtone Warm Peer (Topic Prefix: ${topicPrefix}) ---`)

    const baseDir = path.join(os.homedir(), '.dialtone', 'swarm', 'warm')
    fs.mkdirSync(baseDir, { recursive: true })

    const log = new AutoLog({
        topic: topicPrefix + '-log',
        storage: path.join(baseDir, 'log'),
        requireBootstrap: false // Acts as a potential host/seed
    })

    const kv = new AutoKV({
        topic: topicPrefix + '-kv',
        storage: path.join(baseDir, 'kv'),
        requireBootstrap: false
    })

    console.log('[warm] Initializing log...')
    await log.ready()
    console.log('[warm] Initializing kv...')
    await kv.ready()

    console.log('[warm] Peer is now ACTIVE and seeding topics.')
    console.log('[warm] Press Ctrl+C to stop.')

    // Keep alive
    setInterval(() => {
        const logPeers = log.swarm.connections.size
        const kvPeers = kv.swarm.connections.size
        console.log(`[warm] Status: Log Peers: ${logPeers}, KV Peers: ${kvPeers}`)
    }, 10000)

    Bare.on('sigint', async () => {
        console.log('\n[warm] Shutting down...')
        await log.close()
        await kv.close()
        Bare.exit(0)
    })
}

main().catch(err => {
    console.error('[warm] Fatal Error:', err)
    Bare.exit(1)
})
