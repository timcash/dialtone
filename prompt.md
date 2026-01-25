1. make a small change to the ticket plugin. 
2. when we do `dialtone.sh ticket done <ticket-name>`
3. it should verfiy all code is checked into git for the branch
4. it should verify all ## SUBTASK: in the ticket are marked done
4. it should not run any tests
5. it should make sure the `dialtone.sh github pr` is set to ready

now improve `src/plugins/test/test/test_suite.go` to show 
1. `./dialtone.sh test plugin`
2. `./dialtone.sh test ticket test-test-tags`
3. `./dialtone.sh test tags <tag-one> <tag-two> <tag-three>`

1. wait tags should work like `dialtone.sh test tags metadata camera-filters red-team`
2. <tag-one> <tag-two> <tag-three> could be in any order in this case <tag-one> is `metadata`, <tag-two> is `camera-filters` and <tag-three> is `red-team`
3. but in this case `dialtone.sh test tags camera-filters red-team` <tag-one> is `camera-filters` <tag-two> is `red-team` and `metadata` is not specified

update the docs to better reflect this and use more standard cli documenation for optional list of tags

I updated the test system
```bash
# Run all subtask tests for the specific ticket
./dialtone.sh test ticket <ticket-name>

# Or run a specific subtask test
./dialtone.sh test ticket <ticket-name> --subtask <subtask-name>

# If you worked on a plugin you can run the plugin test
./dialtone.sh test plugin <plugin-name>

# Mark tests with tags so you can use them later as a list of space separated tags
./dialtone.sh test tags tag-one tag-two tag-three

# Add --list to see the list of tests that would run to any command e.g.
./dialtone.sh test ticket <ticket-name> --list
./dialtone.sh test tags tag-one tag-two --list
```

can you make sure the ai tests use this new system. see `tickets/test-test-tags/test/test.go` for an example of how to use tags
show using the `test plugin` command to run our current ai plugin tests

