-- One-time code from the UI. The bot redeems /start <code> into chat_id.

SET @has_link := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_telegram' AND COLUMN_NAME = 'link_code'
);
SET @sql_link := IF(@has_link = 0,
  'ALTER TABLE user_telegram ADD COLUMN link_code VARCHAR(16) NOT NULL DEFAULT ''''',
  'SELECT 1');
PREPARE stmt FROM @sql_link;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_idx := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_telegram' AND INDEX_NAME = 'idx_user_telegram_link'
);
SET @sql_idx := IF(@has_idx = 0,
  'ALTER TABLE user_telegram ADD KEY idx_user_telegram_link (link_code)',
  'SELECT 1');
PREPARE stmt FROM @sql_idx;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
