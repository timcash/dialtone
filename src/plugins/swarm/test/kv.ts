import Autobase from 'autobase';
import Hyperbee from 'hyperbee';
import Hypercore from 'hypercore';
import fs from 'fs';
import path from 'path';
import os from 'os';
import b4a from 'b4a';

function createTmpDir() {
    const tmp = path.join(os.tmpdir(), 'dialtone-kv-' + Math.random().toString(16).slice(2));
    fs.mkdirSync(tmp, { recursive: true });
    return tmp;
}

// ----------------------------------------------------------------------------
// Autobee: A peer-to-peer K/V store using Autobase + Hyperbee
// ----------------------------------------------------------------------------
class Autobee {
    base: any;
    bee: any;

    constructor(localCore: any, inputs: any[]) {
        // Autobase: Causal ordering layer
        this.base = new Autobase({
            inputs,
            localInput: localCore,
        });

        // Hyperbee: B-tree index layer using the linearized view
        this.bee = new Hyperbee(this.base.view, {
            extension: false,
            keyEncoding: 'utf-8',
            valueEncoding: 'json'
        });
    }

    async put(key: string, value: any) {
        return this.bee.put(key, value);
    }

    async get(key: string) {
        return this.bee.get(key);
    }

    async ready() {
        await this.base.ready();
        await this.bee.ready();
    }
}

// ----------------------------------------------------------------------------
// Simulation
// ----------------------------------------------------------------------------
async function main() {
    console.log('--- Starting Swarm K/V (Autobee) Test [Ephemeral Mode] ---');

    // Peer A
    const dirA = createTmpDir();
    const coreA = new Hypercore(dirA);
    await coreA.ready();
    console.log(`Peer A: ${b4a.toString(coreA.key, 'hex').slice(0, 8)}... `);

    // Peer B
    const dirB = createTmpDir();
    const coreB = new Hypercore(dirB);
    await coreB.ready();
    console.log(`Peer B: ${b4a.toString(coreB.key, 'hex').slice(0, 8)}... `);

    // Setup Autobase inputs (both peers know each other)
    const inputs = [coreA, coreB];

    // Create two views of the same "system"
    const dbA = new Autobee(coreA, inputs);
    const dbB = new Autobee(coreB, inputs);

    await dbA.ready();
    await dbB.ready();
    console.log('Databases ready.');

    // ---------------------------------------------------------
    // Scenario 1: Sequential Write/Read
    // ---------------------------------------------------------
    console.log('\n[Scenario 1] Sequential Write');
    await dbA.put('status', 'online');
    console.log('Peer A wrote "status" = "online"');

    // Sync
    let s1 = dbA.base.replicate(true);
    let s2 = dbB.base.replicate(false);
    s1.pipe(s2).pipe(s1);

    // Sync
    await new Promise(pkg => setTimeout(pkg, 100));
    s1.destroy(); s2.destroy();

    const ans = await dbB.get('status');
    console.log(`Peer B read "status": "${ans ? ans.value : 'null'}"`);

    if (ans && ans.value === 'online') {
        console.log('SUCCESS: Data synced from A to B');
    } else {
        console.error('FAILURE: Data did not sync');
    }

    // ---------------------------------------------------------
    // Scenario 2: Concurrent Writes (Convergence)
    // ---------------------------------------------------------
    console.log('\n[Scenario 2] Concurrent Writes (Convergence)');

    // Both write to DIFFERENT keys at the same time (logically) without syncing yet
    console.log('Peer A writes "config.a" = 1');
    const p1 = dbA.put('config.a', 1);

    console.log('Peer B writes "config.b" = 2');
    const p2 = dbB.put('config.b', 2);

    await Promise.all([p1, p2]);

    // Sync again
    console.log('Syncing...');
    s1 = dbA.base.replicate(true);
    s2 = dbB.base.replicate(false);
    s1.pipe(s2).pipe(s1);
    await new Promise(r => setTimeout(r, 100));
    s1.destroy(); s2.destroy();

    // Verify State Convergence
    console.log('Verifying state...');
    const valA1 = (await dbA.get('config.a'))?.value;
    const valA2 = (await dbA.get('config.b'))?.value;

    const valB1 = (await dbB.get('config.a'))?.value;
    const valB2 = (await dbB.get('config.b'))?.value;

    console.log(`Peer A Sees: a=${valA1}, b=${valA2}`);
    console.log(`Peer B Sees: a=${valB1}, b=${valB2}`);

    if (valA1 === 1 && valA2 === 2 && valB1 === 1 && valB2 === 2) {
        console.log('SUCCESS: Both peers converged to the same state (Union of all writes)');
    } else {
        console.error('FAILURE: State did not converge correctly');
    }

    // Cleanup
    console.log('Cleaning up...');
    await coreA.close();
    await coreB.close();
    fs.rmSync(dirA, { recursive: true, force: true });
    fs.rmSync(dirB, { recursive: true, force: true });
    console.log('Temporary directories removed.');

    process.exit(0);
}

main().catch(err => {
    console.error(err);
    process.exit(1);
});