CREATE TABLE IF NOT EXISTS face_model_embeddings (
    model_key VARBINARY(64) NOT NULL,
    marker_uid VARBINARY(42) NOT NULL,
    embeddings_json MEDIUMBLOB,
    created_at DATETIME,
    updated_at DATETIME,
    PRIMARY KEY (model_key, marker_uid)
);

CREATE INDEX IF NOT EXISTS idx_face_model_embeddings_marker_uid ON face_model_embeddings (marker_uid);

CREATE TABLE IF NOT EXISTS face_model_clusters (
    id VARBINARY(64) NOT NULL,
    model_key VARBINARY(64) DEFAULT '',
    face_src VARBINARY(8),
    face_kind INTEGER,
    face_hidden BOOLEAN,
    subj_uid VARBINARY(42) DEFAULT '',
    samples INTEGER,
    sample_radius DOUBLE,
    collisions INTEGER,
    collision_radius DOUBLE,
    embedding_json MEDIUMBLOB,
    matched_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME,
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_face_model_clusters_model_key ON face_model_clusters (model_key);
CREATE INDEX IF NOT EXISTS idx_face_model_clusters_subj_uid ON face_model_clusters (subj_uid);

CREATE TABLE IF NOT EXISTS face_model_markers (
    model_key VARBINARY(64) NOT NULL,
    marker_uid VARBINARY(42) NOT NULL,
    face_id VARBINARY(64),
    face_dist DOUBLE DEFAULT -1,
    subj_uid VARBINARY(42) DEFAULT '',
    subj_src VARBINARY(8) DEFAULT '',
    matched_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME,
    PRIMARY KEY (model_key, marker_uid)
);

CREATE INDEX IF NOT EXISTS idx_face_model_markers_face_id ON face_model_markers (face_id);
CREATE INDEX IF NOT EXISTS idx_face_model_markers_subj_uid ON face_model_markers (subj_uid);
CREATE INDEX IF NOT EXISTS idx_face_model_markers_matched_at ON face_model_markers (matched_at);

CREATE TABLE IF NOT EXISTS face_model_runs (
    model_key VARBINARY(64) NOT NULL,
    status VARBINARY(32) DEFAULT '',
    marker_count INTEGER,
    embedding_count INTEGER,
    cluster_count INTEGER,
    matched_count INTEGER,
    error VARCHAR(255) DEFAULT '',
    prepared_at DATETIME,
    promoted_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME,
    PRIMARY KEY (model_key)
);
