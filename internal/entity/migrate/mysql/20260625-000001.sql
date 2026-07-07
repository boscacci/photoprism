CREATE TABLE IF NOT EXISTS face_model_embeddings (
    model_key VARBINARY(64) NOT NULL,
    marker_uid VARBINARY(42) NOT NULL,
    embeddings_json MEDIUMBLOB,
    created_at DATETIME,
    updated_at DATETIME,
    PRIMARY KEY (model_key, marker_uid),
    INDEX idx_face_model_embeddings_marker_uid (marker_uid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS face_model_clusters (
    id VARBINARY(64) NOT NULL,
    model_key VARBINARY(64) DEFAULT '',
    face_src VARBINARY(8),
    face_kind INT,
    face_hidden BOOLEAN,
    subj_uid VARBINARY(42) DEFAULT '',
    samples INT,
    sample_radius DOUBLE,
    collisions INT,
    collision_radius DOUBLE,
    embedding_json MEDIUMBLOB,
    matched_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME,
    PRIMARY KEY (id),
    INDEX idx_face_model_clusters_model_key (model_key),
    INDEX idx_face_model_clusters_subj_uid (subj_uid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

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
    PRIMARY KEY (model_key, marker_uid),
    INDEX idx_face_model_markers_face_id (face_id),
    INDEX idx_face_model_markers_subj_uid (subj_uid),
    INDEX idx_face_model_markers_matched_at (matched_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS face_model_runs (
    model_key VARBINARY(64) NOT NULL,
    status VARBINARY(32) DEFAULT '',
    marker_count INT,
    embedding_count INT,
    cluster_count INT,
    matched_count INT,
    error VARCHAR(255) DEFAULT '',
    prepared_at DATETIME,
    promoted_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME,
    PRIMARY KEY (model_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
