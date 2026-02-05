# Ticket system
- read `src/plugins/ticket/README.md`
- use the ticket system. 
- alert if something is confusing or not working.

# Task 1:
solve the following issue 

### issue:198
- title: create holepunch bare js-runtime integration
- labels: 
```markdown
see https://github.com/holepunchto/bare

Docs and Requirements 
1. allows dialtone to start and connect to predefined topic
2. keeps a local duckdb of key/content-hash mappings with metadata
3. possibly uses an addon https://github.com/holepunchto/bare-addon
4. see netcode here https://github.com/holepunchto/hyperdht
5. something on native building here https://github.com/holepunchto/bare-native
6. to see a list of all bare modules https://github.com/orgs/holepunchto/repositories?language=&q=bare&sort=&type=all
7. https://github.com/holepunchto/bare-net
8. use https://github.com/holepunchto/bare-build to build for native
9. example of building an app https://github.com/holepunchto/keet-appling-next

I would like this plugin to allow a `./dialtone.sh swarm <topic>` command 
it should use this as a minimal network code with bare 

https://github.com/holepunchto/hyperdht

this does not need to integrate with any golang code other than the CLI to start it. 
as a plugin it can have its own install and build that dev.go can call via CLI 

# LLM 2 Prompt:
- add a `./dialtone.sh swarm test` command to test the swarm plugin.

# LLM 2 Response:
I have added the `./dialtone.sh swarm test` command. This command executes a dedicated test script (`src/plugins/swarm/app/test.js`) using the Pear runtime.

### How to Run the Test:
```bash
./dialtone.sh swarm test
```
The test will attempt to join a test topic and exit successfully upon establishing a connection.


# LLM 3 Prompt:
- I want to test having two different nodes join with different private keys but communicate over the same topic.

# LLM 3 Response:
I have updated the `./dialtone.sh swarm test` command to perform a multi-peer integration test. 

- **Test Logic**: The command now spawns two parallel Pear processes (`peer-a` and `peer-b`).
- **Key Derivation**: Each peer derives a unique 32-byte key pair from its name using `hypercore-crypto.hash`.
- **Communication**: Both peers join the `test-topic`, exchange `ping/pong` messages, and exit successfully upon verifying the connection.

### Run the Integration Test:
```bash
./dialtone.sh swarm test
```


