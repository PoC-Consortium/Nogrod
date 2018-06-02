CREATE TABLE IF NOT EXISTS `transaction` (
  `id` BIGINT(20) unsigned NOT NULL,
  `amount` BIGINT(20) NOT NULL,
  `recipient_id` BIGINT(20) unsigned NOT NULL,
  `created` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  INDEX `account_fk_idx` (`recipient_id` ASC)
)
ENGINE = InnoDB;