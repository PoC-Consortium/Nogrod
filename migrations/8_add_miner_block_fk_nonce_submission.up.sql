CALL `proc_foreign_key_not_exists`(
	'nonce_submission', 
    'miner_block_fk', 
    '
ALTER TABLE `nonce_submission`
ADD CONSTRAINT `miner_block_fk`
    FOREIGN KEY (`block_height`)
    REFERENCES `block` (`height`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
');