can you get the current issues with the `./dialtone.sh github` plugin ? 

write them to prompt.md under the `# LLM:1` on line 4 using markdown
# LLM:1

### Current Issues with `./dialtone.sh github` Plugin

1.  **Resolved**: Critical build failure due to missing `nexttone` plugin has been fixed by purging `nexttone` references.
2.  **Resolved**: Hardcoded Vercel path and `check-deploy` logic have been removed from the GitHub plugin.
3.  **Resolved**: Inconsistent CLI naming has been fixed by standardizing on `./dialtone.sh` across the repo.
4.  **Resolved**: Deprecated `issue sync` command has been removed from the GitHub plugin.
5.  **Resolved**: Robust pre-flight checks for `gh` have been implemented. The plugin now prompts the user to run `./dialtone.sh github install` if the binary is missing.

# LLM:2 github test
Run `./dialtone.sh plugin test github` to verify the full GitHub PR lifecycle. The test creates a temporary branch, makes a commit, pushes to remote, creates a draft PR, marks it as ready, and then cleans up (closes PR and deletes remote/local branch).

# Issue Format

### issue:id
title:
> labels:
body

# LLM:3
### issue:201
title: upgrade ai gemini plugin
> labels: 
use a streaming loops like this 
```javascript 
import { GoogleGenAI } from '@google/genai';

const client = new GoogleGenAI({});

const stream = await client.interactions.create({
    model: 'gemini-3-flash-preview',
    input: 'Explain quantum entanglement in simple terms.',
    stream: true,
});

for await (const chunk of stream) {
    if (chunk.event_type === 'content.delta') {
        if (chunk.delta.type === 'text' && 'text' in chunk.delta) {
            process.stdout.write(chunk.delta.text);
        } else if (chunk.delta.type === 'thought' && 'thought' in chunk.delta) {
            process.stdout.write(chunk.delta.thought);
        }
    } else if (chunk.event_type === 'interaction.complete') {
        console.log('\n\n--- Stream Finished ---');
        console.log(`Total Tokens: ${chunk.interaction.usage.total_tokens}`);
    }
}
```
see https://ai.google.dev/gemini-api/docs/interactions?ua=chat#streaming

### issue:200
title: create plugin for web based urdf-studio
> labels: 
see https://github.com/OpenLegged/URDF-Studio

### issue:199
title: create plugin for codex SDK
> labels: 
see https://github.com/openai/codex/tree/main/sdk/typescript
and use the streaming api https://ai.google.dev/gemini-api/docs/interactions?ua=chat#streaming


### issue:198
title: create holepunch bare js-runtime integration
> labels: 
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




### issue:138
title: create a ticket for a plugin to build geospatial maps
> labels: 
use https://github.com/milos-agathon/3d-bivariate-climate-maps for examples

### issue:137
title: review this paper for camera based navigation
> labels: 
https://arxiv.org/pdf/2601.19887

extract key ideas and create a summary in this issue as a comment

### issue:117
title: integrate a simple plugin with duckdb and graph queries
> labels: 
use https://duckdb.org/2025/10/22/duckdb-graph-queries-duckpgq for inspiration

### issue:104
title: improve the install plugin for dev and production
> labels: p0, ticket, ready
1. research improve the install plugin to have dev and production install paths for each plugin 
2. move each plugin install and build logic (if they have any) into the plugin
3. use a install.go and build.go in the plugin folder if needed
4. test installing different configurations on linux macos windows-wsl and rasberry pi

### issue:97
title: ticket integrate a bubbletea plugin
> labels: bug, enhancement, ticket, p1, ready
see https://github.com/charmbracelet/bubbletea
and give dialtone a TUI

### issue:96
title: integrate ideas for memory from clawdbot
> labels: question
see post https://x.com/manthanguptaa/status/2015780646770323543?s=46

### issue:91
title: ticket create a rf-detr plugin and demo
> labels: ticket, ready
integrate https://github.com/roboflow/rf-detr into a plugin

use veo3 or newer to generate a video of a cat to fine tune the model

show using the live camera feed to track a cat

### issue:90
title: ticket create a plugin for raspberry pi management
> labels: question

### issue:89
title: ticket integrate machine learning and blender tools into a plugin
> labels: ticket, ready
use this repo for inspiration https://github.com/Fugtemypt123/VIGA

use blender apis to directly code 3D scenes

### issue:83
title: ticket integrate opencode cli into robot ui xterm.js element
> labels: ticket, ready
1. turn all prompts or issues into subtasks
1. write the subtask TEST first 
1. then write the code to pass the test
1. try to use only `dialtone.sh` and `git` commands
1. DO NOT run multiple cli commands in one line e.g. `dialtone.sh deploy`
1. exceptions for searching code and the web and editing code directly are acceptable
1. guide all work into `dialtone.sh ticket subtask` commands with a test at the end
1. build plugins if you do not see one you think fits this subtask
1. you may REORDER subtasks if needed
1. use the `dialtone.sh help` to print the help menu

## SUBTASK: allow the robot rover web ui to stream the opencode cli into xterm.js
## SUBTASK: look at the web ui that gets deployed to the robot
## SUBTASK: also look at the webpage interface that comes with opencode
## deploy the code to the robot at the end to verify the discord integration works from the robot

### issue:82
title: plugin: integrate discord api at https://discord.com/developers/social-sdk
> labels: ticket, ready
1. Create a new ticket called discord-api-simple
2. use the ticket workflow

## SUBTASK: use the documentation
1. look here to get started https://discord.com/developers/social-sdk

### issue:55
title: Create atopile plugin
> labels: ticket, ready
see https://docs.atopile.io/atopile/introduction

Introduction
Design circuit boards blazing fast - with code
atopile brings the power of software development workflows to hardware design. By describing electronics with code, you can leverage, modularity version control, and deep validation.
Capture design intelligence and constraints directly in your code, enabling auto-selection of components, embedded calculations checked on every build, and reliable, configurable modules.
This allows for rapid iteration, easier collaboration, and robust designs validated through continuous integration.