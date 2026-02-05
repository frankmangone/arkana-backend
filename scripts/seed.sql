-- Seed script for development: creates sample posts
-- Run with: make seed

-- Insert sample posts with various path identifiers
INSERT OR IGNORE INTO posts (path_identifier, like_count) VALUES
    ('the-zk-chronicles/computation-models', 0),
    ('the-zk-chronicles/circuits-part-2', 0),
    ('the-zk-chronicles/multilinear-extensions', 0),
    ('the-zk-chronicles/circuits-part-1', 0),
    ('the-zk-chronicles/sum-check', 0),
    ('the-zk-chronicles/math-foundations', 0),
    ('the-zk-chronicles/first-steps', 0);

-- Optionally insert a test wallet (for manual testing)
-- Address is a common test address - replace with your own if needed
INSERT OR IGNORE INTO wallets (address, system) VALUES
    ('0x0000000000000000000000000000000000000001', 'ethereum');

-- Verify seeded data
SELECT 'Posts seeded:' as info, COUNT(*) as count FROM posts;
SELECT 'Wallets seeded:' as info, COUNT(*) as count FROM wallets;
