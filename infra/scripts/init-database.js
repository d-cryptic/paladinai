// MongoDB Database Initialization Script for PaladinAI Checkpointing
// This script sets up the database, collections, and indexes for LangGraph checkpointing

print('Starting database initialization for PaladinAI checkpointing...');

// Get database name from environment or use default
var dbName = process.env.MONGODB_DATABASE || 'paladinai_checkpoints';

print('Database name:', dbName);
print('Note: LangGraph uses its own collection names (checkpoints_aio and checkpoint_writes_aio)');

// Switch to the target database
db = db.getSiblingDB(dbName);

// LangGraph AsyncMongoDBSaver uses these specific collection names
var checkpointsCollection = 'checkpoints_aio';
var writesCollection = 'checkpoint_writes_aio';

// Create the checkpoints collection if it doesn't exist
if (!db.getCollectionNames().includes(checkpointsCollection)) {
    print('Creating collection:', checkpointsCollection);
    db.createCollection(checkpointsCollection);
} else {
    print('Collection already exists:', checkpointsCollection);
}

// Create the checkpoint writes collection if it doesn't exist
if (!db.getCollectionNames().includes(writesCollection)) {
    print('Creating collection:', writesCollection);
    db.createCollection(writesCollection);
} else {
    print('Collection already exists:', writesCollection);
}

// Create indexes for checkpoints_aio collection
print('\nCreating indexes for checkpoints_aio collection...');
var checkpointsCol = db.getCollection(checkpointsCollection);

// Index for thread_id and checkpoint_ns (most common query pattern)
try {
    checkpointsCol.createIndex(
        { "thread_id": 1, "checkpoint_ns": 1, "checkpoint_id": 1 },
        { 
            name: "thread_ns_checkpoint_idx",
            background: true 
        }
    );
    print('Created index: thread_ns_checkpoint_idx');
} catch (e) {
    print('Index thread_ns_checkpoint_idx may already exist:', e.message);
}

// Index for parent checkpoint lookups
try {
    checkpointsCol.createIndex(
        { "parent_checkpoint_id": 1 },
        { 
            name: "parent_checkpoint_idx",
            background: true,
            sparse: true 
        }
    );
    print('Created index: parent_checkpoint_idx');
} catch (e) {
    print('Index parent_checkpoint_idx may already exist:', e.message);
}

// Index for type queries
try {
    checkpointsCol.createIndex(
        { "type": 1 },
        { 
            name: "type_idx",
            background: true 
        }
    );
    print('Created index: type_idx');
} catch (e) {
    print('Index type_idx may already exist:', e.message);
}

// Create indexes for checkpoint_writes_aio collection
print('\nCreating indexes for checkpoint_writes_aio collection...');
var writesCol = db.getCollection(writesCollection);

// Index for thread_id, checkpoint_ns and checkpoint_id
try {
    writesCol.createIndex(
        { "thread_id": 1, "checkpoint_ns": 1, "checkpoint_id": 1 },
        { 
            name: "thread_ns_checkpoint_writes_idx",
            background: true 
        }
    );
    print('Created index: thread_ns_checkpoint_writes_idx');
} catch (e) {
    print('Index thread_ns_checkpoint_writes_idx may already exist:', e.message);
}

// Index for task_id
try {
    writesCol.createIndex(
        { "task_id": 1 },
        { 
            name: "task_id_idx",
            background: true 
        }
    );
    print('Created index: task_id_idx');
} catch (e) {
    print('Index task_id_idx may already exist:', e.message);
}

// Index for channel queries
try {
    writesCol.createIndex(
        { "channel": 1 },
        { 
            name: "channel_idx",
            background: true 
        }
    );
    print('Created index: channel_idx');
} catch (e) {
    print('Index channel_idx may already exist:', e.message);
}

// Note: LangGraph manages its own TTL and cleanup, so we don't create TTL indexes

// Display collection stats
print('\nCollection statistics:');

// Stats for checkpoints_aio
try {
    var stats = db.runCommand({ collStats: checkpointsCollection });
    print('\n' + checkpointsCollection + ':');
    print('  Document count:', stats.count);
    print('  Storage size:', stats.storageSize, 'bytes');
    print('  Index count:', stats.nindexes);
} catch (e) {
    print('Could not retrieve stats for', checkpointsCollection + ':', e.message);
}

// Stats for checkpoint_writes_aio
try {
    var stats = db.runCommand({ collStats: writesCollection });
    print('\n' + writesCollection + ':');
    print('  Document count:', stats.count);
    print('  Storage size:', stats.storageSize, 'bytes');
    print('  Index count:', stats.nindexes);
} catch (e) {
    print('Could not retrieve stats for', writesCollection + ':', e.message);
}

// List all collections in the database
print('\nAll collections in database:');
try {
    var collections = db.getCollectionNames();
    collections.forEach(function(col) {
        print('  -', col);
    });
} catch (e) {
    print('Could not list collections:', e.message);
}

print('\nDatabase initialization completed successfully');
print('PaladinAI checkpointing database is ready for use with LangGraph AsyncMongoDBSaver');