CREATE TABLE IF NOT EXISTS detections (
    id               BIGSERIAL        PRIMARY KEY,
    node_id          VARCHAR(64)      NOT NULL,
    detected_at      TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    detection_type   VARCHAR(32)      NOT NULL,
    src_ip           INET             NOT NULL,
    dst_ip           INET             NOT NULL,
    src_port         INTEGER          NOT NULL,
    dst_port         INTEGER          NOT NULL,
    protocol         VARCHAR(8)       NOT NULL,
    flow_start       TIMESTAMPTZ,
    flow_end         TIMESTAMPTZ,
    fwd_packets      INTEGER,
    bwd_packets      INTEGER,
    duration_us      BIGINT,
    local_score      DOUBLE PRECISION,
    features         DOUBLE PRECISION[],
    yes_votes        INTEGER,
    total_votes      INTEGER,
    connection_count INTEGER,
    consensus_ms     BIGINT
);

CREATE INDEX IF NOT EXISTS idx_detections_detected_at
    ON detections (detected_at DESC);

CREATE INDEX IF NOT EXISTS idx_detections_node_id
    ON detections (node_id, detected_at DESC);

CREATE INDEX IF NOT EXISTS idx_detections_src_ip
    ON detections (src_ip);

CREATE INDEX IF NOT EXISTS idx_detections_type
    ON detections (detection_type);

CREATE TABLE IF NOT EXISTS consensus_votes (
    id              BIGSERIAL        PRIMARY KEY,
    detection_id    BIGINT           NOT NULL REFERENCES detections(id) ON DELETE CASCADE,
    voter_node_id   VARCHAR(64)      NOT NULL,
    vote            BOOLEAN          NOT NULL,
    score           DOUBLE PRECISION NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_consensus_votes_detection
    ON consensus_votes (detection_id);

CREATE TABLE IF NOT EXISTS node_health (
    id               BIGSERIAL   PRIMARY KEY,
    node_id          VARCHAR(64) NOT NULL,
    checked_by       VARCHAR(64) NOT NULL,
    checked_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reachable        BOOLEAN     NOT NULL,
    response_time_ms INTEGER
);

CREATE INDEX IF NOT EXISTS idx_node_health_node_id
    ON node_health (node_id, checked_at DESC);
