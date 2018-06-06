CREATE PROCEDURE `proc_foreign_key_not_exists` (IN tableName VARCHAR(64), IN constraintName VARCHAR(64), IN statement VARCHAR(1000))
BEGIN
	IF NOT EXISTS(
            SELECT * FROM information_schema.table_constraints
            WHERE
                table_schema    = DATABASE()     AND
                table_name      = tableName      AND
                constraint_name = constraintName AND
                constraint_type = 'FOREIGN KEY')
        THEN
			SET @sql = statement;
            PREPARE stmt FROM @sql;
            EXECUTE stmt;
            DEALLOCATE PREPARE stmt;
        END IF;
END
