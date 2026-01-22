# Ticket Improvement Task
1. This is a method to improve and extend tickets
2. Tickets may start as simple notes or ideas dictacted to a noisy room
2. Always focus on building subtasks that can be tested with simple golang code
3. Use Test Driven Development Terminology and Techniques to guide Large Language Models


## Example of a quick ticket that needs to be improved
== BEGIN EXAMPLE TICKET ==
```markdown
# Branch: jax-demo-plugin
# Task: Implement a simple JAX plugin
# 

## Subtask
1. go to this url and read the docs https://docs.jax.dev/en/latest/notebooks/thinking_in_jax.html
2. using a geospatial theme and dataset (you can create fake data) 
3. create an outline of the plugin and what it will demonstrate
4. DO NOT CODE yet just make the outline

use the template ticket at `./tickets/template-ticket/ticket.md` to guide improvements to this ticket 

think about how that pluging could be build step by step using tests to guide the work. 

then fill in a new ticket. you can put code samples in the markdown 

also add docs to `./docs/vendor/jax.md` about jax to help future work as you go

also mention that the user should use `pixi` and python to do the code learn more about pixi here @pixi.md 

tests are always written in golang

make sure the subtasks can be used to guide a Large Language Model
```
== END EXAMPLE TICKET ==