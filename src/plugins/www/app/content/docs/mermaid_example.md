# Example Mermaid Diagram
```mermaid
---
config:
  theme: dark
---
flowchart TD
  A[node1]
  B[node2]
  C[node3]
  D["$$x(t)=c_1\begin{bmatrix}-\cos{t}+\sin{t}\\ 2\cos{t} \end{bmatrix}e^{2t}$$"]
  E[node4]
  subgraph cluster_1
    A
    B
    C
  end
  A e1@--> E
  B --> C
  B --> D
  C --> D
  D --> E
  classDef green1 stroke:#00FF00,stroke-width:5px;
  classDef animate stroke-dasharray: 9,5,stroke-dashoffset: 900,animation: dash 25s linear infinite;  
  class D green1;
  class e1 animate;