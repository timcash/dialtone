# graphs and networks

```
1. nodes
2. edges
3. paths
4. cycles
5. connected components
```

# mermaid diagram
1. nodes are the vertices of the graph
2. edges are the connections between the nodes
3. paths are the sequences of nodes and edges
4. cycles are the paths that start and end at the same node
5. connected components are the subgraphs that are connected to each other

## highlight one edge to be green
```mermaid
---
config:
  layout: elk
  theme: dark
---
flowchart LR
  A@{ shape: circ, label: "Stop" }
  B@{ shape: circ, label: "Continue" }
  A e1@--> B
  classDef green stroke:#00FF00,stroke-width:5px;
  class e1 green;
```

## subgraph with colored node and math equations for the label
```mermaid
---
config:
  layout: elk
  theme: dark
---
flowchart
  A[node1]
  B[node2]
  C[node3]
  D["$$x(t)=c_1\begin{bmatrix}-\cos{t}+\sin{t}\\ 2\cos{t} \end{bmatrix}e^{2t}$$"]
  E[node4]
  subgraph cluster_1
    A --> B
    B --> C
    B --> D
    C --> D
  end
  E --> D
  classDef green1 stroke:#00FF00,stroke-width:5px;
  class D green1;
```