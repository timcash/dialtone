-- Create nodes
SELECT cypher_execute('CREATE (r:Router {name: "Core-Router-1", ip: "10.0.0.1"})');
SELECT cypher_execute('CREATE (s:Switch {name: "Access-Switch-A", mac: "00:1A:2B:3C:4D:5E"})');
SELECT cypher_execute('CREATE (h1:Host {name: "Server-Web-1", ip: "10.0.1.10"})');
SELECT cypher_execute('CREATE (h2:Host {name: "Server-DB-1", ip: "10.0.1.20"})');

-- Create edges
SELECT cypher_execute('MATCH (r:Router {name: "Core-Router-1"}), (s:Switch {name: "Access-Switch-A"}) CREATE (r)-[:UPLINK {speed: "10G"}]->(s)');
SELECT cypher_execute('MATCH (s:Switch {name: "Access-Switch-A"}), (h:Host {name: "Server-Web-1"}) CREATE (s)-[:CONNECTED_TO {port: "FastEthernet0/1"}]->(h)');
SELECT cypher_execute('MATCH (s:Switch {name: "Access-Switch-A"}), (h:Host {name: "Server-DB-1"}) CREATE (s)-[:CONNECTED_TO {port: "FastEthernet0/2"}]->(h)');

-- Query the network
SELECT cypher_execute('MATCH (r:Router)-[:UPLINK]->(s:Switch)-[:CONNECTED_TO]->(h:Host) RETURN r.name, s.name, h.name, h.ip');
