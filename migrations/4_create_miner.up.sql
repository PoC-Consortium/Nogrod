CREATE TABLE IF NOT EXISTS `miner` (
  `id` BIGINT(20) unsigned NOT NULL,
  `capacity` BIGINT(20) NULL,
  PRIMARY KEY (`id`)
)
ENGINE = InnoDB;