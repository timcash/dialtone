SELECT cypher_execute('CREATE (r1:Router {name: "R1"})');
SELECT cypher_execute('CREATE (r2:Router {name: "R2"})');
SELECT cypher_execute('CREATE (r3:Router {name: "R3"})');
SELECT cypher_execute('CREATE (r4:Router {name: "R4"})');
SELECT cypher_execute('MATCH (r1:Router {name: "R1"}), (r2:Router {name: "R2"}) CREATE (r1)-[:LINK {weight: 10}]->(r2)');
SELECT cypher_execute('MATCH (r2:Router {name: "R2"}), (r3:Router {name: "R3"}) CREATE (r2)-[:LINK {weight: 5}]->(r3)');
SELECT cypher_execute('MATCH (r1:Router {name: "R1"}), (r4:Router {name: "R4"}) CREATE (r1)-[:LINK {weight: 2}]->(r4)');
SELECT cypher_execute('MATCH (r4:Router {name: "R4"}), (r3:Router {name: "R3"}) CREATE (r4)-[:LINK {weight: 2}]->(r3)');

-- The extension has graphDijkstra in C, but let's see if it's exposed as a SQL function
-- We saw graph_shortest_path (which uses BFS for unweighted graphs)
-- Let's try graph_dijkstra or similar
