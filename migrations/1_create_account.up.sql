CREATE TABLE IF NOT EXISTS `account` (
  `id` BIGINT(20) unsigned NOT NULL,
  `address` VARCHAR(20) NOT NULL,
  `name` VARCHAR(100),
  `pending` BIGINT(20) NOT NULL DEFAULT 0,
  `min_payout_value` BIGINT(20),
  `payout_interval` VARCHAR(20),
  `next_payout_date` DATETIME,
  PRIMARY KEY (`id`),
  UNIQUE INDEX `address_UNIQUE` (`address` ASC))
ENGINE = InnoDB
DEFAULT CHARACTER SET = utf8;