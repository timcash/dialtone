/ticket start working on a geminicli ai plugin

it should use `./dialtone.sh ai --gemini "summarize this README.md" kind of api

add this to the already existing ai plugin

make a new install and build.go file inside the plugin folder. 

after you make the ticket turn this all into subtasks before starting 


make install plugin docs look more like the ticket plugin docs

map out the architecture of how install works

make suggestions for how to change the system to allow for 
1. install --dev : for installing all development dependencies like go, npm
2. 


look at code in the github plugin

improve this workflow doc for LLM agents to use 

it should guide them through using the `./dialtone.sh` cli tool to 
1. list the current issues with tags in a clean simple markdown compatible printout 
2. show details on any tagged p0 first
3. decide if it can improve or ask a question by adding a new comment on that priority set
4. move down to p1 issues and decide if it can improve or ask any good questions on that set
5. and continue working through each ticket until it has either marked them with a ready tag or run out of time

the goal of this workflow is to get issues into `ticket` ready format. That means it has all the validated material for another agent to come work on it like
1. in the correct ticket format
2. has meaningful subtasks with great test suggestions
3. each subtask is not too large and can be done with a small code change

expand on these ideas now

# ticket-format-updates
add a new ticket about updating all documentation to show ALL tests commands must use the following formats


# replace progress.txt with `dialtone.sh ticket next`
1. remove progress.txt from the repo docs and all code
2. update all workflows to mention that using `dialtone.sh ticket next` for each subtask is mandatory
3. `dialtone.sh ticket next` runs all current ticket tests making the ticket.md the source of truth for testings
3. `dialtone.sh ticket next` validates the ticket and prints any errors
4. `dialtone.sh ticket next` prints the next subtask to work on
5. `dialtone.sh ticket next` marks the previous subtask as done
6. `dialtone.sh ticket next` is the central building block for all ticket workflows
7. calling `dialtone.sh ticket next` on main branch prints an error and instrucks LLM agents to ask the user for the next ticket.

