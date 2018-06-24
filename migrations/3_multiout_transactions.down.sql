START TRANSACTION;

RENAME TABLE `transaction` TO `transaction_old`;

CREATE TABLE IF NOT EXISTS `transaction` (
  `id` BIGINT(20) unsigned NOT NULL,
  `amount` BIGINT(20) NOT NULL,
  `recipient_id` BIGINT(20) unsigned NOT NULL,
  `created` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  INDEX `account_fk_idx` (`recipient_id` ASC)
)
ENGINE = InnoDB;

-- this is dirty, multiouts cannot be downgraded
INSERT IGNORE INTO `transaction` (id, amount, recipient_id, created)
SELECT t.transaction_id, r.amount, r.recipient_id, t.created
FROM `transaction_old` t JOIN transaction_recipient r ON t.id = r.transaction_id
WHERE t.transaction_id IS NOT NULL;

DROP TABLE IF EXISTS `transaction_recipient`;
DROP TABLE IF EXISTS `transaction_old`;

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
