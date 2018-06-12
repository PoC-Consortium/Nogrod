START TRANSACTION;

ALTER TABLE `transaction`
ADD COLUMN `recipient_id` BIGINT(20) unsigned,
ADD COLUMN `amount` BIGINT(20);

UPDATE `transaction` t, `transaction_recipient` tr SET
t.recipient_id = tr.recipient_id,
t.amount = tr.amount
WHERE t.id = tr.transaction_id;

ALTER TABLE `transaction`
DROP FOREIGN KEY `block_fk`,
DROP `block_height`,
MODIFY `amount` BIGINT(20) NOT NULL,
MODIFY `recipient_id` BIGINT(20) unsigned NOT NULL,
ADD INDEX (`account_fk_idx`) (`recipient_id` ASC);

DROP TABLE IF EXISTS `transaction_recipient`;

CALL `proc_foreign_key_check`(
	'transaction',
    'account_fk',
    '
ALTER TABLE `transaction`
ADD CONSTRAINT `account_fk`
    FOREIGN KEY (`recipient_id`)
    REFERENCES `account` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
', false);

COMMIT;
