// MongoDB Replica Set Initialization Script for PaladinAI
// This script initializes a single-node replica set required for LangGraph checkpointing

print("Starting replica set initialization...");

try {
  // Check if replica set is already initialized
  var status = rs.status();
  print("Replica set already initialized:", JSON.stringify(status, null, 2));
} catch (e) {
  print("Replica set not initialized, proceeding with initialization...");

  // Initialize replica set with single node
  var config = {
    _id: "rs0",
    version: 1,
    members: [
      {
        _id: 0,
        host: "mongodb:27017",
        priority: 1,
      },
    ],
  };

  print(
    "Initializing replica set with config:",
    JSON.stringify(config, null, 2),
  );

  var result = rs.initiate(config);
  print("Replica set initiation result:", JSON.stringify(result, null, 2));

  if (result.ok === 1) {
    print("Replica set initialized successfully");

    // Wait for replica set to be ready
    print("Waiting for replica set to become ready...");
    var maxAttempts = 30;
    var attempt = 0;

    while (attempt < maxAttempts) {
      try {
        var status = rs.status();
        if (
          status.members &&
          status.members[0] &&
          status.members[0].state === 1
        ) {
          print("Replica set is ready and primary is elected");
          break;
        }
        print("Waiting for primary election... attempt", attempt + 1);
        sleep(1000); // Wait 1 second
        attempt++;
      } catch (e) {
        print("Error checking replica set status:", e.message);
        sleep(1000);
        attempt++;
      }
    }

    if (attempt >= maxAttempts) {
      print("Warning: Replica set initialization may not be complete");
    }
  } else {
    print("Error: Failed to initialize replica set:", result);
  }
}

print("Replica set initialization script completed");
