CREATE TABLE IF NOT EXISTS user_telegram (
  user_id    CHAR(36)    NOT NULL,
  chat_id    VARCHAR(32) NOT NULL DEFAULT '',
  link_code  VARCHAR(16) NOT NULL DEFAULT '',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (user_id),
  KEY idx_user_telegram_link (link_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
