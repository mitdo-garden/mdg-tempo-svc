CREATE TABLE tempos (
  id            CHAR(36)     NOT NULL,
  user_id       CHAR(36)     NOT NULL,
  name          VARCHAR(255) NOT NULL,
  content       MEDIUMTEXT   NOT NULL,
  confidence    TINYINT      NOT NULL DEFAULT 3,
  due           DATETIME(3)  NOT NULL,
  ease          DOUBLE       NOT NULL DEFAULT 2.3,
  interval_days DOUBLE       NOT NULL DEFAULT 0,
  reminder      JSON         NOT NULL,
  scale         DOUBLE       NOT NULL DEFAULT 1,
  created_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_tempos_user (user_id),
  KEY idx_tempos_due (due)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE examples (
  id         CHAR(36)     NOT NULL,
  tempo_id   CHAR(36)     NOT NULL,
  text       TEXT         NOT NULL,
  sort_order INT          NOT NULL DEFAULT 0,
  created_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_examples_tempo (tempo_id),
  CONSTRAINT fk_examples_tempo FOREIGN KEY (tempo_id) REFERENCES tempos (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE media (
  id         CHAR(36)      NOT NULL,
  tempo_id   CHAR(36)      NOT NULL,
  kind       VARCHAR(16)   NOT NULL,
  url        VARCHAR(2048) NOT NULL,
  title      VARCHAR(255)  NOT NULL DEFAULT '',
  sort_order INT           NOT NULL DEFAULT 0,
  created_at DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_media_tempo (tempo_id),
  CONSTRAINT fk_media_tempo FOREIGN KEY (tempo_id) REFERENCES tempos (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE tempo_tags (
  tempo_id CHAR(36)    NOT NULL,
  tag      VARCHAR(64) NOT NULL,
  PRIMARY KEY (tempo_id, tag),
  KEY idx_tempo_tags_tag (tag),
  CONSTRAINT fk_tempo_tags_tempo FOREIGN KEY (tempo_id) REFERENCES tempos (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE review_history (
  id          CHAR(36)     NOT NULL,
  user_id     CHAR(36)     NOT NULL,
  tempo_id    CHAR(36)     NOT NULL,
  rating      TINYINT      NOT NULL,
  reviewed_at DATETIME(3)  NOT NULL,
  PRIMARY KEY (id),
  KEY idx_history_user (user_id),
  KEY idx_history_tempo (tempo_id),
  KEY idx_history_at (reviewed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE activity_log (
  id         CHAR(36)     NOT NULL,
  user_id    CHAR(36)     NOT NULL,
  tempo_id   CHAR(36)     NOT NULL,
  channel    VARCHAR(32)  NOT NULL,
  status     VARCHAR(32)  NOT NULL,
  created_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_activity_user (user_id),
  KEY idx_activity_tempo (tempo_id),
  KEY idx_activity_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
