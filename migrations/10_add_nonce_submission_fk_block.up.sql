CALL `proc_foreign_key_not_exists`(
	'block',
    'nonce_submission_fk',
    '
ALTER TABLE `block`
ADD CONSTRAINT `nonce_submission_fk`
    FOREIGN KEY (`best_nonce_submission_id`)
    REFERENCES `nonce_submission` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
');
