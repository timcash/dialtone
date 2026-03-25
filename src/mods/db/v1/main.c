#include <stdio.h>
#include <stdlib.h>
#include "sqlite3.h"

// Declare the sqlite-graph extension's initialization function.
// When compiling statically, we must declare this so we can pass it to SQLite.
extern int sqlite3_graph_init(sqlite3 *db, char **pzErrMsg, const sqlite3_api_routines *pApi);

// Callback function to print query results
int print_callback(void *NotUsed, int argc, char **argv, char **azColName) {
    for (int i = 0; i < argc; i++) {
        printf("%s = %s\n", azColName[i], argv[i] ? argv[i] : "NULL");
    }
    return 0;
}

int main(int argc, char **argv) {
    sqlite3 *db;
    char *err_msg = 0;
    int rc;

    // 1. Register the sqlite-graph extension statically
    // This ensures that every new SQLite connection automatically loads the graph capabilities.
    rc = sqlite3_auto_extension((void (*)(void))sqlite3_graph_init);
    if (rc != SQLITE_OK) {
        fprintf(stderr, "Failed to register static graph extension.\n");
        return 1;
    }

    // 2. Open an in-memory database (or specify a file like "graph.db")
    rc = sqlite3_open(":memory:", &db);
    if (rc != SQLITE_OK) {
        fprintf(stderr, "Cannot open database: %s\n", sqlite3_errmsg(db));
        return 1;
    }

    printf("Successfully opened SQLite database with Graph extension embedded.\n\n");

    // 3. Initialize the graph virtual table
    rc = sqlite3_exec(db, "CREATE VIRTUAL TABLE mygraph USING graph()", 0, 0, &err_msg);
    if (rc != SQLITE_OK) {
        fprintf(stderr, "Failed to create graph table: %s\n", err_msg);
        sqlite3_free(err_msg);
        sqlite3_close(db);
        return 1;
    }

    // 4. Run Cypher Queries: Create Nodes
    const char *create_nodes_cypher = 
        "SELECT cypher_execute('CREATE (r:Router {name: \"Core-Router-1\", ip: \"10.0.0.1\"})');"
        "SELECT cypher_execute('CREATE (s:Switch {name: \"Access-Switch-A\", mac: \"00:1A:2B:3C:4D:5E\"})');"
        "SELECT cypher_execute('CREATE (h1:Host {name: \"Server-Web-1\", ip: \"10.0.1.10\"})');"
        "SELECT cypher_execute('CREATE (h2:Host {name: \"Server-DB-1\", ip: \"10.0.1.20\"})');";

    rc = sqlite3_exec(db, create_nodes_cypher, 0, 0, &err_msg);
    if (rc != SQLITE_OK) {
        fprintf(stderr, "Cypher Node Creation Error: %s\n", err_msg);
        sqlite3_free(err_msg);
    } else {
        printf("Created network nodes: Router, Switch, and two Hosts.\n");
    }

    // 5. Run Cypher Queries: Create Edges (Relationships)
    const char *create_edges_cypher = 
        "SELECT cypher_execute('MATCH (r:Router {name: \"Core-Router-1\"}), (s:Switch {name: \"Access-Switch-A\"}) CREATE (r)-[:UPLINK {speed: \"10G\"}]->(s)');"
        "SELECT cypher_execute('MATCH (s:Switch {name: \"Access-Switch-A\"}), (h:Host {name: \"Server-Web-1\"}) CREATE (s)-[:CONNECTED_TO {port: \"FastEthernet0/1\"}]->(h)');"
        "SELECT cypher_execute('MATCH (s:Switch {name: \"Access-Switch-A\"}), (h:Host {name: \"Server-DB-1\"}) CREATE (s)-[:CONNECTED_TO {port: \"FastEthernet0/2\"}]->(h)');";

    rc = sqlite3_exec(db, create_edges_cypher, 0, 0, &err_msg);
    if (rc != SQLITE_OK) {
        fprintf(stderr, "Cypher Edge Creation Error: %s\n", err_msg);
        sqlite3_free(err_msg);
    } else {
        printf("Created network topology: Core-Router-1 -> Access-Switch-A -> Servers.\n\n");
    }

    // 6. Run a Cypher MATCH Query and print results
    printf("Querying for Hosts connected to the Switch:\n");
    const char *query_cypher = "SELECT cypher_execute('MATCH (s:Switch)-[:CONNECTED_TO]->(h:Host) RETURN s.name, h.name, h.ip')";
    
    rc = sqlite3_exec(db, query_cypher, print_callback, 0, &err_msg);
    if (rc != SQLITE_OK) {
        fprintf(stderr, "Cypher Match Error: %s\n", err_msg);
        sqlite3_free(err_msg);
    }
    
    printf("\n");

    // 7. Graph Algorithms (e.g., Centrality via SQL functions provided by the extension)
    printf("Running Graph Algorithm: Degree Centrality on the network nodes\n");
    const char *algo_query = "SELECT id, properties, graph_degree_centrality(id) as centrality FROM mygraph_nodes";
    rc = sqlite3_exec(db, algo_query, print_callback, 0, &err_msg);
    if (rc != SQLITE_OK) {
        fprintf(stderr, "Algorithm Error: %s\n", err_msg);
        sqlite3_free(err_msg);
    }

    // Cleanup
    sqlite3_close(db);
    return 0;
}
