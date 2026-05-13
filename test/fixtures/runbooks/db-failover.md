# Database Failover Runbook

## Overview

This runbook covers the steps to promote a Postgres replica when the primary fails.

## Prerequisites

- Access to the bastion host
- PagerDuty on-call rotation access
- RDS console access

## Steps

### Step 1: Detect the Failure

- Check CloudWatch: RDS_DatabaseConnections drops to 0
- Confirm via: `psql -h db-primary -c "SELECT 1"`

### Step 2: Promote the Replica

```bash
aws rds promote-read-replica --db-instance-identifier db-replica-01
```

### Step 3: Update DNS

- Update `db.internal` CNAME from `db-primary-01` to `db-replica-01`
- TTL should be 30s for faster failover

### Step 4: Notify Stakeholders

- Page on-call engineer
- Post in #incidents channel

## Rollback

If the replica promotion fails, contact the database team immediately.
