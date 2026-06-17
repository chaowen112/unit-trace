CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS clients (
    id              BIGSERIAL PRIMARY KEY,
    client_id       UUID UNIQUE NOT NULL,
    display_name    TEXT NOT NULL DEFAULT 'Unknown',
    first_seen_at   TIMESTAMPTZ,
    last_seen_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tracked_units (
    id                          BIGSERIAL PRIMARY KEY,
    source                      TEXT,
    property_type               TEXT,
    project_name                TEXT,
    address_text                TEXT,
    district                    TEXT,
    bedrooms                    INT,
    first_seen_at               TIMESTAMPTZ,
    last_seen_at                TIMESTAMPTZ,
    last_visited_at             TIMESTAMPTZ,
    first_seen_by_client_id     BIGINT REFERENCES clients(id),
    last_seen_by_client_id      BIGINT REFERENCES clients(id),
    last_visited_by_client_id   BIGINT REFERENCES clients(id),
    first_seen_price            BIGINT,
    current_price               BIGINT,
    lowest_seen_price           BIGINT,
    highest_seen_price          BIGINT,
    possible_relist_count       INT DEFAULT 0,
    snapshot_count              INT DEFAULT 0,
    visit_count                 INT DEFAULT 0,
    interest_label              TEXT,
    updated_at                  TIMESTAMPTZ DEFAULT now(),
    created_at                  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS listing_snapshots (
    id                      BIGSERIAL PRIMARY KEY,
    source                  TEXT NOT NULL,
    listing_url             TEXT NOT NULL,
    canonical_url           TEXT,
    listing_id              TEXT,
    captured_at             TIMESTAMPTZ NOT NULL,
    captured_by_client_id   BIGINT REFERENCES clients(id),
    title                   TEXT,
    asking_price            BIGINT,
    property_type           TEXT,
    project_name            TEXT,
    address_text            TEXT,
    district                TEXT,
    bedrooms                INT,
    bathrooms               INT,
    floor_area              NUMERIC(8,2),
    floor_level_text        TEXT,
    agent_name              TEXT,
    agency_name             TEXT,
    description_text        TEXT,
    description_hash        TEXT,
    image_set_hash          TEXT,
    content_hash            TEXT,
    raw_payload             JSONB,
    created_at              TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tracked_unit_snapshots (
    id              BIGSERIAL PRIMARY KEY,
    tracked_unit_id BIGINT NOT NULL REFERENCES tracked_units(id),
    snapshot_id     BIGINT NOT NULL REFERENCES listing_snapshots(id),
    match_type      TEXT NOT NULL,
    match_score     INT,
    match_reasons   JSONB,
    created_at      TIMESTAMPTZ DEFAULT now(),
    UNIQUE(tracked_unit_id, snapshot_id)
);

CREATE TABLE IF NOT EXISTS listing_visits (
    id              BIGSERIAL PRIMARY KEY,
    tracked_unit_id BIGINT NOT NULL REFERENCES tracked_units(id),
    snapshot_id     BIGINT REFERENCES listing_snapshots(id),
    client_id       BIGINT REFERENCES clients(id),
    source          TEXT NOT NULL,
    listing_url     TEXT NOT NULL,
    listing_id      TEXT,
    visited_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS price_events (
    id              BIGSERIAL PRIMARY KEY,
    tracked_unit_id BIGINT NOT NULL REFERENCES tracked_units(id),
    snapshot_id     BIGINT NOT NULL REFERENCES listing_snapshots(id),
    event_type      TEXT NOT NULL,
    old_price       BIGINT,
    new_price       BIGINT,
    price_delta     BIGINT,
    price_delta_pct NUMERIC(8,4),
    detected_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS images (
    id              BIGSERIAL PRIMARY KEY,
    original_url    TEXT,
    storage_path    TEXT NOT NULL,
    sha256_hash     TEXT NOT NULL UNIQUE,
    phash           TEXT,
    width           INT,
    height          INT,
    content_type    TEXT,
    file_size       BIGINT,
    first_seen_at   TIMESTAMPTZ DEFAULT now(),
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS listing_snapshot_images (
    id              BIGSERIAL PRIMARY KEY,
    snapshot_id     BIGINT NOT NULL REFERENCES listing_snapshots(id),
    image_id        BIGINT NOT NULL REFERENCES images(id),
    sort_order      INT,
    original_url    TEXT,
    created_at      TIMESTAMPTZ DEFAULT now(),
    UNIQUE(snapshot_id, image_id)
);

CREATE TABLE IF NOT EXISTS notes (
    id                  BIGSERIAL PRIMARY KEY,
    tracked_unit_id     BIGINT NOT NULL REFERENCES tracked_units(id),
    author_client_id    BIGINT REFERENCES clients(id),
    note                TEXT NOT NULL,
    created_at          TIMESTAMPTZ DEFAULT now()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_listing_snapshots_listing_id
    ON listing_snapshots(source, listing_id)
    WHERE listing_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_listing_snapshots_canonical_url
    ON listing_snapshots(canonical_url)
    WHERE canonical_url IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tus_unit
    ON tracked_unit_snapshots(tracked_unit_id);

CREATE INDEX IF NOT EXISTS idx_tus_snapshot
    ON tracked_unit_snapshots(snapshot_id);

CREATE INDEX IF NOT EXISTS idx_visits_unit
    ON listing_visits(tracked_unit_id);

CREATE INDEX IF NOT EXISTS idx_visits_client
    ON listing_visits(client_id);

CREATE INDEX IF NOT EXISTS idx_price_events_unit
    ON price_events(tracked_unit_id);

CREATE INDEX IF NOT EXISTS idx_tracked_units_district
    ON tracked_units(district)
    WHERE district IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_images_sha256
    ON images(sha256_hash);

CREATE INDEX IF NOT EXISTS idx_notes_unit
    ON notes(tracked_unit_id);

CREATE INDEX IF NOT EXISTS idx_listing_snapshots_desc_trgm
    ON listing_snapshots USING gin(description_text gin_trgm_ops)
    WHERE description_text IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_listing_snapshots_title_trgm
    ON listing_snapshots USING gin(title gin_trgm_ops)
    WHERE title IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tracked_units_project
    ON tracked_units(project_name)
    WHERE project_name IS NOT NULL;
