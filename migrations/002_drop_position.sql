-- Drop map coordinates only. Tempo rows and every other column stay.

SET @has_x := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tempos' AND COLUMN_NAME = 'pos_x'
);
SET @sql_x := IF(@has_x > 0, 'ALTER TABLE tempos DROP COLUMN pos_x', 'SELECT 1');
PREPARE stmt FROM @sql_x;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_y := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tempos' AND COLUMN_NAME = 'pos_y'
);
SET @sql_y := IF(@has_y > 0, 'ALTER TABLE tempos DROP COLUMN pos_y', 'SELECT 1');
PREPARE stmt FROM @sql_y;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
