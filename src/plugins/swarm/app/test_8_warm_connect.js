import { AutoLog } from './autolog_v2.js'
import { AutoKV } from './autokv_v2.js'
import path from 'bare-path'
import os from 'bare-os'
import fs from 'bare-fs'
import b4a from 'b4a'

async function main () {
  console.log('--- Test: Connect to Warm Peer ---')
  const topic = 'dialtone-v2'
  const logBootstrap = '3a2d390dd39a21e0ecb1095f4ddaf2f6384e470be3ab98975e559b4a514ff9ac'
  const kvBootstrap = 'f0558ddc0caea18e6b77712aefcab9c2b43a6fda70318db98b4ef9ab35ed30ef'
  
  const baseDir = path.join(os.tmpdir(), 'dt-warm-connect-' + Date.now())

  const log = new AutoLog({
    topic: topic + '-log',
    storage: path.join(baseDir, 'log'),
    bootstrap: logBootstrap,
    logId: 'CLIENT-LOG'
  })

  const kv = new AutoKV({
    topic: topic + '-kv',
    storage: path.join(baseDir, 'kv'),
    bootstrap: kvBootstrap,
    logId: 'CLIENT-KV'
  })

  console.log('[test] Initializing client...')
  await Promise.all([log.ready(), kv.ready()])

  console.log('[test] Waiting for writability...')
  await Promise.all([log.waitWritable(), kv.waitWritable()])
  console.log('[test] Client is writable!')

  await log.append({ msg: 'Hello from client log' })
  await kv.put('status', { msg: 'Hello from client kv' })

  console.log('[test] Verifying KV persistence...')
  const val = await kv.get('status')
  console.log('[test] KV status:', val)

  if (val?.msg === 'Hello from client kv') {
    console.log('--- [PASS] WARM CONNECT SUCCESS ---')
  } else {
    console.error('--- [FAIL] WARM CONNECT MISMATCH ---')
  }

  await log.close()
  await kv.close()
  Bare.exit(0)
}

main().catch(err => {
  console.error(err)
  Bare.exit(1)
})