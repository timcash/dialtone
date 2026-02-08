import { AutoLog } from './autolog_v2.js'
import { AutoKV } from './autokv_v2.js'
import path from 'bare-path'
import os from 'bare-os'
import fs from 'bare-fs'
import b4a from 'b4a'

/**
 * Level 4: Full V2 Lifecycle.
 * Uses the actual production classes.
 */

async function main () {
  console.log('--- Level 4: Full V2 Lifecycle ---')
  const topic = 'level4-test-' + Date.now()
  const baseDir = path.join(os.tmpdir(), 'dt-level4-' + Date.now())

  // Node A (Bootstrap)
  const logA = new AutoLog({
    topic: topic + '-log',
    storage: path.join(baseDir, 'a', 'log'),
    logId: 'A'
  })
  await logA.ready()
  const logBootstrap = b4a.toString(logA.base.key, 'hex')

  // Node B (Follower)
  const logB = new AutoLog({
    topic: topic + '-log',
    storage: path.join(baseDir, 'b', 'log'),
    bootstrap: logBootstrap,
    logId: 'B',
    pulseInterval: 1000
  })
  await logB.ready()

  console.log('[test] Waiting for B to become writable...')
  const start = Date.now()
  while (!logB.base.writable) {
    if (Date.now() - start > 15000) throw new Error('Timeout waiting for V2 auth')
    await new Promise(r => setTimeout(r, 500))
  }
  console.log('[test] Node B is writable!')

  await logB.append({ msg: 'Level 4 Success' })
  
  console.log('[test] Syncing...')
  const syncStart = Date.now()
  let result = null
  while (Date.now() - syncStart < 10000) {
    await logA.base.update()
    const list = await logA.list()
    result = list.find(l => l.data?.msg === 'Level 4 Success')
    if (result) break
    await new Promise(r => setTimeout(r, 500))
  }

  if (result) {
    console.log('--- [PASS] LEVEL 4 SUCCESS ---')
  } else {
    console.error('--- [FAIL] LEVEL 4 SYNC TIMEOUT ---')
  }

  await logA.close()
  await logB.close()
  Bare.exit(0)
}

main().catch(err => {
    console.error(err)
    Bare.exit(1)
})
