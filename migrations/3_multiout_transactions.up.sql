START TRANSACTION;

CREATE TABLE IF NOT EXISTS `transaction_recipient` (
  `id` BIGINT(20) NOT NULL AUTO_INCREMENT,
  `transaction_id` BIGINT(20) unsigned NOT NULL,
  `recipient_id` BIGINT(20) unsigned NOT NULL,
  `amount` BIGINT(20) NOT NULL,
  PRIMARY KEY (`id`),
  INDEX `account_fk_idx` (`recipient_id` ASC),
  INDEX `transaction_fk_idx` (`transaction_id` ASC)
)
ENGINE = InnoDB;

INSERT INTO `transaction_recipient` (`transaction_id`, `recipient_id`, `amount`)
SELECT `id,` `recipient_id,` `amount`
FROM `transaction`;

ALTER TABLE `transaction`
DROP FOREIGN KEY `account_fk`,
DROP `recipient_id`,
DROP `amount`,
ADD COLUMN `block_height` BIGINT(20) unsigned,
ADD INDEX `block_fk_idx` (`block_height` ASC);

CALL `proc_foreign_key_check`(
	'transaction_recipient',
    'account_fk',
    '
ALTER TABLE `transaction_recipient`
ADD CONSTRAINT `account_fk`
    FOREIGN KEY (`recipient_id`)
    REFERENCES `account` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
', false);

CALL `proc_foreign_key_check`(
	'transaction_recipient',
    'transaction_fk',
    '
ALTER TABLE `transaction_recipient`
ADD CONSTRAINT `transaction_fk`
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
