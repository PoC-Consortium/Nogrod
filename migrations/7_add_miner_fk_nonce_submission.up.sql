CALL `proc_foreign_key_not_exists`(
	'nonce_submission', 
    'miner_fk', 
    '
ALTER TABLE `nonce_submission`
ADD CONSTRAINT `miner_fk`
    FOREIGN KEY (`miner_id`)
    REFERENCES `account` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
');