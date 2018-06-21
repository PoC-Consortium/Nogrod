START TRANSACTION;

CREATE TABLE IF NOT EXISTS `transaction_recipient` (
  `id` BIGINT(20) NOT NULL AUTO_INCREMENT,
  `transaction_id` BIGINT(20) NOT NULL,
  `recipient_id` BIGINT(20) unsigned NOT NULL,
  `amount` BIGINT(20) NOT NULL,
  PRIMARY KEY (`id`),
  INDEX `account_fk_idx` (`recipient_id` ASC),
  INDEX `transaction_fk_idx` (`transaction_id` ASC)
)
ENGINE = InnoDB;

RENAME TABLE `transaction` TO `transaction_old`;

CREATE TABLE IF NOT EXISTS `transaction` (
  `id` BIGINT(20) NOT NULL AUTO_INCREMENT,
  `transaction_id` BIGINT(20) unsigned,
  `block_height` BIGINT(20) unsigned,
  `created` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  INDEX `transaction_block_fk_idx` (`block_height` ASC)
)
ENGINE = InnoDB;

INSERT INTO `transaction` (transaction_id, created)
SELECT id, created
FROM transaction_old;

INSERT INTO transaction_recipient (transaction_id, recipient_id, amount)
SELECT t.id, o.recipient_id, o.amount
FROM `transaction` t JOIN transaction_old AS o ON transaction_id = o.id;

DROP TABLE transaction_old;

CALL `proc_foreign_key_check`(
	'transaction_recipient',
    'transaction_recipient_account_fk',
    '
ALTER TABLE `transaction_recipient`
ADD CONSTRAINT `transaction_recipient_account_fk`
    FOREIGN KEY (`recipient_id`)
    REFERENCES `account` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
', false);

CALL `proc_foreign_key_check`(
	'transaction_recipient',
    'transaction_recipient_transaction_fk',
    '
ALTER TABLE `transaction_recipient`
ADD CONSTRAINT `transaction_recipient_transaction_fk`
    FOREIGN KEY (`transaction_id`)
    REFERENCES `transaction` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
', false);

CALL `proc_foreign_key_check`(
	'transaction',
  'transaction_block_fk',
  '
ALTER TABLE `transaction`
ADD CONSTRAINT `transaction_block_fk`
    FOREIGN KEY (`block_height`)
    REFERENCES `block` (`height`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
', false);

COMMIT;
