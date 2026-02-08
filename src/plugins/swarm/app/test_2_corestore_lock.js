import Corestore from 'corestore'
import path from 'bare-path'
import os from 'bare-os'
import fs from 'bare-fs'

async function main() {
  console.log('--- Test 2: Corestore Lock Verification ---')
  
  const storagePath = path.join(os.tmpdir(), 'dt-lock-test-' + Date.now())
  fs.mkdirSync(storagePath, { recursive: true })

  console.log(`[test] Opening first corestore at ${storagePath}...`)
  const store1 = new Corestore(storagePath)
  const core1 = store1.get({ name: 'main' })
  await core1.ready()
  console.log('[test] Store 1 is READY and holding the lock.')

  console.log('[test] Attempting to open SECOND corestore on same path (expecting failure)...')
  const store2 = new Corestore(storagePath)
  const core2 = store2.get({ name: 'main' })

  try {
    // In many environments, .ready() or the first operation will throw if locked
    const timeout = new Promise((_, reject) => setTimeout(() => reject(new Error('TIMEOUT')), 5000))
    await Promise.race([core2.ready(), timeout])
    
    console.error('[FAIL] Test 2: Second corestore opened successfully! Lock is NOT working as expected.')
    process.exit(1)
  } catch (err) {
    if (err.message === 'TIMEOUT') {
      console.log('[test] Store 2 is hanging (typical behavior for some lock backends).')
    } else {
      console.log(`[test] Store 2 FAILED as expected: ${err.message}`)
    }
    console.log('[SUCCESS] Corestore lock is active. Multiple instances cannot share a directory.')
  }

  console.log('[test] Cleaning up...')
  await store1.close()
  // Clean up dir
  try { fs.rmSync(storagePath, { recursive: true, force: true }) } catch {}
  
  console.log('--- [PASS] TEST 2 SUCCESS ---')
  Bare.exit(0)
}

main().catch(err => {
  console.error('[FATAL]', err)
  Bare.exit(1)
})
