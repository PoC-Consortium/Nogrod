CALL `proc_foreign_key_not_exists`(
	'transaction',
    'account_fk',
    '
ALTER TABLE `transaction`
ADD CONSTRAINT `account_fk`
    FOREIGN KEY (`recipient_id`)
    REFERENCES `account` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
');
