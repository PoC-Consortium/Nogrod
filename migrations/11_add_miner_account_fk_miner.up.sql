CALL `proc_foreign_key_not_exists`(
	'miner',
    'miner_account_fk',
    '
ALTER TABLE `miner`
ADD CONSTRAINT `miner_account_fk`
    FOREIGN KEY (`id`)
    REFERENCES `account` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
');
