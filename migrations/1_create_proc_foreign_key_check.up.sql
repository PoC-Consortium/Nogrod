DROP procedure IF EXISTS `proc_foreign_key_check`;
CREATE PROCEDURE `proc_foreign_key_check`(IN tableName VARCHAR(64), IN constraintName VARCHAR(64), IN statement VARCHAR(1000), IN exsts BOOLEAN)
BEGIN
	SET @check = (
		SELECT constraint_name FROM information_schema.table_constraints
		WHERE
			table_schema    = DATABASE()     AND
			table_name      = tableName      AND
			constraint_name = constraintName AND
			constraint_type = 'FOREIGN KEY');

	SELECT (exsts AND @check IS NOT NULL) OR (NOT exsts AND @check IS NULL);
	IF (exsts AND @check IS NOT NULL) OR (NOT exsts AND @check IS NULL)
        THEN
			SET @sql = statement;
            PREPARE stmt FROM @sql;
            EXECUTE stmt;
            DEALLOCATE PREPARE stmt;
        END IF;
END