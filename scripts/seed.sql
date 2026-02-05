-- Seed script: creates all posts from content folder
-- Run with: make seed

-- blockchain-101
INSERT OR IGNORE INTO posts (path_identifier, like_count) VALUES
    ('blockchain-101/a-primer-on-consensus', 0),
    ('blockchain-101/beyond-the-blockchain-part-1', 0),
    ('blockchain-101/beyond-the-blockchain-part-2', 0),
    ('blockchain-101/blockchain-safari', 0),
    ('blockchain-101/consensus-revisited', 0),
    ('blockchain-101/coretime', 0),
    ('blockchain-101/ethereum', 0),
    ('blockchain-101/evolving-a-blockchain', 0),
    ('blockchain-101/handling-data', 0),
    ('blockchain-101/how-it-all-began', 0),
    ('blockchain-101/jam', 0),
    ('blockchain-101/parallelizing-execution', 0),
    ('blockchain-101/polkadot-consensus', 0),
    ('blockchain-101/polkadot', 0),
    ('blockchain-101/rollups', 0),
    ('blockchain-101/smart-contracts-part-2', 0),
    ('blockchain-101/smart-contracts', 0),
    ('blockchain-101/solana-programs', 0),
    ('blockchain-101/solana', 0),
    ('blockchain-101/storage', 0),
    ('blockchain-101/transactions', 0),
    ('blockchain-101/wrapping-up-bitcoin', 0),
    ('blockchain-101/wrapping-up-ethereum', 0),
    ('blockchain-101/zk-in-blockchain', 0);

-- cryptography-101
INSERT OR IGNORE INTO posts (path_identifier, like_count) VALUES
    ('cryptography-101/arithmetic-circuits', 0),
    ('cryptography-101/asides-evaluating-security', 0),
    ('cryptography-101/asides-rsa-explained', 0),
    ('cryptography-101/commitment-schemes-revisited', 0),
    ('cryptography-101/elliptic-curves-somewhat-demystified', 0),
    ('cryptography-101/encryption-and-digital-signatures', 0),
    ('cryptography-101/fully-homomorphic-encryption', 0),
    ('cryptography-101/hashing', 0),
    ('cryptography-101/homomorphisms-and-isomorphisms', 0),
    ('cryptography-101/pairing-applications-and-more', 0),
    ('cryptography-101/pairings', 0),
    ('cryptography-101/polynomials', 0),
    ('cryptography-101/post-quantum-cryptography', 0),
    ('cryptography-101/protocols-galore', 0),
    ('cryptography-101/ring-learning-with-errors', 0),
    ('cryptography-101/rings', 0),
    ('cryptography-101/signatures-recharged', 0),
    ('cryptography-101/starks', 0),
    ('cryptography-101/threshold-signatures', 0),
    ('cryptography-101/where-to-start', 0),
    ('cryptography-101/zero-knowledge-proofs-part-1', 0),
    ('cryptography-101/zero-knowledge-proofs-part-2', 0),
    ('cryptography-101/zero-knowledge-proofs-part-3', 0);

-- elliptic-curves-in-depth
INSERT OR IGNORE INTO posts (path_identifier, like_count) VALUES
    ('elliptic-curves-in-depth/part-1', 0),
    ('elliptic-curves-in-depth/part-2', 0),
    ('elliptic-curves-in-depth/part-3', 0),
    ('elliptic-curves-in-depth/part-4', 0),
    ('elliptic-curves-in-depth/part-5', 0),
    ('elliptic-curves-in-depth/part-6', 0),
    ('elliptic-curves-in-depth/part-7', 0),
    ('elliptic-curves-in-depth/part-8', 0),
    ('elliptic-curves-in-depth/part-9', 0),
    ('elliptic-curves-in-depth/part-10', 0);

-- ethereum
INSERT OR IGNORE INTO posts (path_identifier, like_count) VALUES
    ('ethereum/the-gem-of-pectra', 0);

-- the-zk-chronicles
INSERT OR IGNORE INTO posts (path_identifier, like_count) VALUES
    ('the-zk-chronicles/circuits-part-1', 0),
    ('the-zk-chronicles/circuits-part-2', 0),
    ('the-zk-chronicles/computation-models', 0),
    ('the-zk-chronicles/first-steps', 0),
    ('the-zk-chronicles/math-foundations', 0),
    ('the-zk-chronicles/multilinear-extensions', 0),
    ('the-zk-chronicles/sum-check', 0);

-- wtf-is
INSERT OR IGNORE INTO posts (path_identifier, like_count) VALUES
    ('wtf-is/quantum-computing', 0),
    ('wtf-is/risc-v', 0),
    ('wtf-is/the-internet', 0);

-- Verify seeded data
SELECT 'Posts seeded:' as info, COUNT(*) as count FROM posts;
SELECT 'Wallets seeded:' as info, COUNT(*) as count FROM wallets;
