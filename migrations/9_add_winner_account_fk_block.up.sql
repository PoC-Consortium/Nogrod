CALL `proc_foreign_key_not_exists`(
	'block',
    'winner_account_fk',
    '
ALTER TABLE `block`
ADD CONSTRAINT `winner_account_fk`
    FOREIGN KEY (`winner_id`)
    REFERENCES `account` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
');
