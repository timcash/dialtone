# DIALTONE (Virtual Librarian)

`DIALTONE:` is a **virtual librarian** that combines a task database, multimedia search engine, global messaging infrastructure, robotic controls and LLM CLI tools into a single interface.

`DIALTONE:`
- forms a structured message stream between `LLM:` and `USER:`
### stream-1: 
`issues` from a `USER/LLM:` is any multimodal prompt input into `./dialtone.sh` 
- after colaborative review an `issue` becomes an `outline` which becomes a `task`
- programtically explores a `task` directed asiclic graph (DAG)
- writes parquet files for flexible and strutured logs
- assigns scores to `LLM:` and `USER:` created plan outlines
- uses the concept of a `budget`, `move-score` and `success_probability`
- compares multiple `LLM:` and `USER:` plans and selects the best one
- routes subtasks to multiple `LLM:` and `USER:` for relative comparison
- injects `subtask` lists into new tickets to act as `workflows`
- wraps duckdb for larger than memeory analysis and search
- dynamically writes context to files with a subtask and `--sign` key
- works with `LLM:` and `USER:` to craft CLI `commands`
- executes `commands` then proxies to `LLM:` and `USER:`
- is designed to bootstrap itself up from a shell file
- uses Golang as a simple but proven at scale interface to computation
- uses TDD to mananage large codebases
- uses NATS as a message bus and data infrastructure interface


### Example Log

```xml
  <turn>
    <user>turn this issue into a ticket `src/issues/0039.md`</user>
    <llm>
      <action>Reading the issue `src/issues/0039.md`</action>
      <reasoning>The issue is about adding a section to the www site that shows the Earth spinning. It appears to have enough information to become a ticket. We should start in add mode.</reasoning>
      <command>./dialtone.sh ticket add www-dev-page-earth-spin</command>
    </llm>
  </turn>
  <turn>
    <dialtone-response id="dt-3X4FT-449NV">
      <message-length>1024 bytes</message-length>
      <mode>add</mode>
      <context>
        <item>the ticket has been added with relavant files</item>
        <item>the files at are `src/tickets/www-dev-page-earth-spin/`</item>
        <item>before `start` or `review` check other tickets for similarity</item>
        <item>use this reference document: `docs/workflows/issue-similarity.md`</item>
        <decision>decide if this issue should be a subtask of an existing open ticket</decision>
        <decision>decied if this issue should be maked as a duplicate</decision>
      </context>

      <next-commands>
        <command>./dialtone.sh ticket list --sign dt-3X4FT-449NV</command>
      </next-commands>
    </dialtone-response>
  </turn>
  <turn>
    <llm>
      <reasoning>The ticket has been added but I need to review other tickets to ensure there is no duplication. I must use the `--sign` to advance the state of the ticket.</reasoning>
      <command>./dialtone.sh ticket list --sign dt-3X4FT-449NV</command>
    </llm>
    <turn>
    <dialtone id="dt-8VCCD-332HN">
      <message-length>2048 bytes</message-length>
      <mode>list</mode>
      <context>
        A ticket summary list has been created. With fields "duplicat" and "insert". 
        Review them for similarity or to potentially add this issue to as a subtask of an existing ticket.
        Use the `--sign` to advance the state of the ticket.
      </context>
      <resource path="src/tickets/ticket-list-summary.md"/>
      <command-list>
        <option type="insert">
          <reasoning>If the issue is not duplicated but does belong in another ticket use this command</reasoning>
          <command>./dialtone.sh ticket --sign yes-8VCCD-332HN</command>
        </option>
        <option type="duplicate">./dialtone.sh ticket --sign no-8VCCD-332HN</option>
      </command-list>
    </dialtone>
  </turn>
  <turn>
    <assistant>
      <reasoning>After reviewing the ticket list, I have determined that there are similar tickets. I will now link this issue as a subtask to the most relevant ticket.</reasoning>
      <command>./dialtone.sh ticket --sign yes-8VCCD-332HN</command>
    </assistant>
    <dialtone>
      <message-length>1024 bytes</message-length>
      <mode>add</mode>
      <context>
        <item>The 
      </context>
      <next-commands>
        <command>./dialtone.sh ticket list --sign dt-3X4FT-449NV</command>
      </next-commands>
    </dialtone>
  </turn>
```

# Ticket: 
`./dialtone.sh ticket` keeps a .ticket-config file in the root of the project with the following fields:
  - `ticket-name`: the name of the ticket
  - `subtask-name`: the name of the subtask

### Ticket: command line interface (CLI)
```shell
./dialtone.sh issue list --offset <offset> --limit <limit>
./dialtone.sh issue add <issue-name>
./dialtone.sh issue comment <issue-name> <comment>
./dialtone.sh ticket list
./dialtone.sh ticket print <ticket-name>
./dialtone.sh ticket add <ticket-name>
./dialtone.sh ticket outline <ticket-name>
./dialtone.sh ticket start <ticket-name>
./dialtone.sh ticket review <ticket-name>
./dialtone.sh ticket subtask list
./dialtone.sh ticket subtask add <name> --desc "..."
./dialtone.sh ticket subtask review
./dialtone.sh ticket subtask start
./dialtone.sh ticket subtask outline
./dialtone.sh ticket subtask testcmd <subtask-name> <test-command...>
./dialtone.sh ticket summary update
./dialtone.sh ticket subtask done <ticket-name> <subtask-name>
./dialtone.sh ticket done
```

### Ticket: modes
- `add`: create a new ticket and then guides a search for similarity or overlap
- `outline`: turn an issue into a ticket outline
- `review`: turn an outline into a ticket
- `start`: execute the ticket while adding new subtasks where neededand checking for errors
- `done`: post ticket review and commit the changes

### Ticket: `add` mode

- `./dialtone.sh ticket add <ticket-name>`
  - **Purpose**: create a new ticket. Use this when you want to create a new ticket.
  - **Outcome**: creates a new ticket and enters the `add` mode.

### Ticket: `outline` mode
- `./dialtone.sh ticket outline <ticket-name>`
- **Purpose**: turn an issue into a ticket outline. Use this when you want to turn an issue into a ticket outline.
- **Outcome**: turns an issue into a ticket outline and enters the `outline` mode.

### Ticket: `review` mode
- `./dialtone.sh ticket review <ticket-name>`
- **Purpose**: prep-only. Use this when you want DIALTONE + the LLM to **review the ticket DB and subtasks** and make sure it’s ready to work on later.
- **Workflow**: DIALTONE iterates over the ticket and each subtask and asks review questions like:
  1. is the goal aligned with subtasks
  2. should there be more subtasks
  3. are any subtasks too large
  4. is there work that should be put into a different ticket because it is not relevant
  5. does this ticket create a new plugin
  6. does this ticket have a update documentation subtask
  7. does this subtask have the correct test-command
- **Skips**: does **not** suggest running tests, reviewing logs, or marking subtasks `done`.
- **Outcome**: the ticket is marked **reviewed** and is ready for `start`.
- **Repeatable**: while in `review` mode, re-run the review iteration at any time with `./dialtone.sh ticket next`.

### Ticket: `start` mode

- `./dialtone.sh ticket start <ticket-name>`
  - **Purpose**: execution. Use this when you are ready to actually do work on the ticket.
  - **Outcome**: enters the normal subtask/verification workflow.

### Ticket: `done` mode

- `./dialtone.sh ticket done <ticket-name>`
  - **Purpose**: finalize. Use this when you are ready to finalize the ticket.
  - **Outcome**: commits the changes and prints the next steps.


### Ticket: state

Tickets track a simple `state` field to indicate where they are in the lifecycle:

- `added`: created but not reviewed
- `outlined`: turned into a ticket outline
- `reviewed`: ticket outline reviewed and ready to start later
- `started`: execution has begun
- `blocked`: blocked waiting on a question/acknowledgement or missing planning info
- `done`: ticket completed and ready to be committed

# subtask
- `added`: subtask created but not reviewed
- `outlined`: subtask turned into a subtask outline
- `reviewed`: subtask outline reviewed and ready to start later
- `started`: subtask execution has begun
- `blocked`: subtask blocked waiting on a question/acknowledgement or missing planning info
- `done`: subtask completed and ready to be committed

### Subtask: modes
- `add`: create a new subtask and then guides a search for similarity or overlap
- `review`: turn an subtask into a subtask outline
- `start`: execute the subtask while adding new subtasks where neededand checking for errors