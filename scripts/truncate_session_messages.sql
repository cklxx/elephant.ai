-- Truncate session messages to prevent JSONB size overflow
-- This script reduces all session messages arrays to the most recent 1000 messages.
-- Run this script to clean up existing sessions after deploying the message limit fix.

-- Backup note: Consider backing up the agent_sessions table before running this script.

-- Update sessions where messages array has more than 1000 elements
UPDATE agent_sessions
SET
    messages = (
        SELECT jsonb_agg(elem)
        FROM (
            SELECT elem
            FROM jsonb_array_elements(messages) WITH ORDINALITY AS t(elem, idx)
            ORDER BY idx DESC
            LIMIT 1000
        ) AS limited
    ),
    updated_at = NOW()
WHERE jsonb_array_length(messages) > 1000;

-- Report how many sessions were truncated
DO $$
DECLARE
    affected_count integer;
BEGIN
    GET DIAGNOSTICS affected_count = ROW_COUNT;
    RAISE NOTICE 'Truncated messages in % sessions', affected_count;
END $$;
