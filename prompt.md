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
