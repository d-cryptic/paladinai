// MongoDB Database Initialization Script for PaladinAI Checkpointing
// This script sets up the database, collections, and indexes for LangGraph checkpointing

print('Starting database initialization for PaladinAI checkpointing...');

// Get database name from environment or use default
var dbName = process.env.MONGODB_DATABASE || 'paladinai_checkpoints';
var collectionName = process.env.MONGODB_COLLECTION || 'langgraph_checkpoints';

print('Database name:', dbName);
print('Collection name:', collectionName);

// Switch to the target database
db = db.getSiblingDB(dbName);

// Create the checkpoints collection if it doesn't exist
if (!db.getCollectionNames().includes(collectionName)) {
    print('Creating collection:', collectionName);
    db.createCollection(collectionName);
} else {
    print('Collection already exists:', collectionName);
}

// Create indexes for optimal checkpoint performance
print('Creating indexes for checkpointing performance...');

var collection = db.getCollection(collectionName);

// Index for thread_id (most common query pattern)
try {
    collection.createIndex(
        { "thread_id": 1, "checkpoint_id": 1 },
        { 
            name: "thread_checkpoint_idx",
            background: true 
        }
    );
    print('Created index: thread_checkpoint_idx');
} catch (e) {
    print('Index thread_checkpoint_idx may already exist:', e.message);
}

// Index for timestamp-based queries (cleanup operations)
try {
    collection.createIndex(
        { "ts": 1 },
        { 
            name: "timestamp_idx",
            background: true 
        }
    );
    print('Created index: timestamp_idx');
} catch (e) {
    print('Index timestamp_idx may already exist:', e.message);
}

// Compound index for thread_id and timestamp (listing checkpoints)
try {
    collection.createIndex(
        { "thread_id": 1, "ts": -1 },
        { 
            name: "thread_timestamp_idx",
            background: true 
        }
    );
    print('Created index: thread_timestamp_idx');
} catch (e) {
    print('Index thread_timestamp_idx may already exist:', e.message);
}

// Index for parent_checkpoint_id (checkpoint hierarchy)
try {
    collection.createIndex(
        { "parent_checkpoint_id": 1 },
        { 
            name: "parent_checkpoint_idx",
            background: true,
            sparse: true  // Only index documents that have this field
        }
    );
    print('Created index: parent_checkpoint_idx');
} catch (e) {
    print('Index parent_checkpoint_idx may already exist:', e.message);
}

// Create a TTL index for automatic cleanup (30 days retention)
try {
    collection.createIndex(
        { "ts": 1 },
        { 
            name: "checkpoint_ttl_idx",
            expireAfterSeconds: 30 * 24 * 60 * 60, // 30 days in seconds
            background: true 
        }
    );
    print('Created TTL index: checkpoint_ttl_idx (30 days retention)');
} catch (e) {
    print('TTL index checkpoint_ttl_idx may already exist:', e.message);
}

// Create a test document to verify everything works
print('Creating test checkpoint document...');
try {
    var testDoc = {
        thread_id: "test-thread-" + new Date().getTime(),
        checkpoint_id: "test-checkpoint-" + new Date().getTime(),
        parent_checkpoint_id: null,
        type: "checkpoint",
        checkpoint: {
            v: 1,
            ts: new Date().toISOString(),
            id: "test-checkpoint-" + new Date().getTime(),
            channel_values: {
                test: true,
                initialized: new Date().toISOString()
            },
            channel_versions: {
                test: 1
            },
            versions_seen: {},
            pending_sends: []
        },
        metadata: {
            source: "init-script",
            step: 0,
            writes: {},
            parents: {}
        },
        ts: new Date()
    };
    
    var result = collection.insertOne(testDoc);
    print('Test document inserted with ID:', result.insertedId);
    
    // Verify we can query it
    var found = collection.findOne({ thread_id: testDoc.thread_id });
    if (found) {
        print('Test document verification successful');
        // Clean up test document
        collection.deleteOne({ _id: result.insertedId });
        print('Test document cleaned up');
    } else {
        print('Warning: Test document verification failed');
    }
} catch (e) {
    print('Error creating test document:', e.message);
}

// Display collection stats
print('Collection statistics:');
try {
    var stats = db.runCommand({ collStats: collectionName });
    print('Document count:', stats.count);
    print('Storage size:', stats.storageSize, 'bytes');
    print('Index count:', stats.nindexes);
} catch (e) {
    print('Could not retrieve collection stats:', e.message);
}

// List all indexes
print('Created indexes:');
try {
    var indexes = collection.getIndexes();
    indexes.forEach(function(index) {
        print('- Index:', index.name, 'on fields:', JSON.stringify(index.key));
    });
} catch (e) {
    print('Could not list indexes:', e.message);
}

print('Database initialization completed successfully');
print('PaladinAI checkpointing database is ready for use');
