import { AutoLog } from './autolog_v2.js'
import { AutoKV } from './autokv_v2.js'
import path from 'bare-path'
import os from 'bare-os'
import fs from 'bare-fs'
import crypto from 'hypercore-crypto'
import b4a from 'b4a'
import Hyperswarm from 'hyperswarm'
import Corestore from 'corestore'

const logFile = path.join(os.homedir(), '.dialtone', 'swarm', 'warm_session.log')
fs.mkdirSync(path.dirname(logFile), { recursive: true })
fs.writeFileSync(logFile, `--- Warm Session Started: ${new Date().toISOString()} ---\n`)

function writeLog(msg) {
  const ts = new Date().toISOString().split('T')[1].slice(0, -1)
  const line = `[${ts}] [warm] ${msg}\n`
  try { fs.appendFileSync(logFile, line) } catch {}
  console.log(line.trim())
}

async function main() {
    const topicPrefix = Bare.argv[Bare.argv.length - 1].endsWith('.js') ? 'dialtone-test' : Bare.argv[Bare.argv.length - 1]
    writeLog(`--- Starting Dialtone Warm Peer (Stable Topic: ${topicPrefix}) ---`)

    const baseDir = path.join(os.homedir(), '.dialtone', 'swarm', 'warm', topicPrefix)
    fs.mkdirSync(baseDir, { recursive: true })

    // GET KEYS AND CLOSE IMMEDIATELY
    const getKeys = async () => {
      const ls = new Corestore(path.join(baseDir, 'log'))
      const ks = new Corestore(path.join(baseDir, 'kv'))
      const lc = ls.get({ name: 'autolog' })
      const kc = ks.get({ name: 'autokv' })
      await Promise.all([lc.ready(), kc.ready()])
      const keys = { log: b4a.toString(lc.key, 'hex'), kv: b4a.toString(kc.key, 'hex') }
      await Promise.all([ls.close(), ks.close()])
      return keys
    }
    
    const bootstrapKeys = await getKeys()
    writeLog(`Bootstrap Keys: Log=0x${bootstrapKeys.log.slice(0,8)}, KV=0x${bootstrapKeys.kv.slice(0,8)}`)

    const log = new AutoLog({
        topic: topicPrefix + '-log',
        storage: path.join(baseDir, 'log'),
        bootstrap: bootstrapKeys.log,
        logId: 'warm-log',
        pulseInterval: 1000,
        swarm: new Hyperswarm({ mdns: true }),
        keySwarm: new Hyperswarm({ mdns: true })
    })

    const kv = new AutoKV({
        topic: topicPrefix + '-kv',
        storage: path.join(baseDir, 'kv'),
        bootstrap: bootstrapKeys.kv,
        logId: 'warm-kv',
        pulseInterval: 1000,
        swarm: new Hyperswarm({ mdns: true }),
        keySwarm: new Hyperswarm({ mdns: true })
    })

    writeLog('Initializing log (V2)...')
    await log.ready()
    writeLog('Initializing kv (V2)...')
    await kv.ready()

    writeLog(`Peer is now ACTIVE for topic: ${topicPrefix}`)
    writeLog('Holding open BOTH Data and Bootstrap topics.')

    // Keep alive and status monitoring
    setInterval(() => {
        const logPeers = log.swarm.connections.size
        const kvPeers = kv.swarm.connections.size
        writeLog(`Status | Peers: (Log:${logPeers}, KV:${kvPeers})`)
    }, 10000)

    Bare.on('sigint', async () => {
        writeLog('Shutting down...')
        await log.close()
        await kv.close()
        Bare.exit(0)
    })
}

main().catch(err => {
    try {
      const errorDetail = err.stack || JSON.stringify(err, null, 2) || err.message || err
      writeLog(`FATAL ERROR: ${errorDetail}`)
    } catch (e) {
      writeLog(`FATAL ERROR: could not serialize error: ${err}`)
    }
    Bare.exit(1)
})