CREATE TABLE IF NOT EXISTS `nonce_submission` (
  `id` BIGINT(20) NOT NULL AUTO_INCREMENT,
  `miner_id` BIGINT(20) unsigned NOT NULL,
  `block_height` BIGINT(20) unsigned NOT NULL,
  `deadline` BIGINT(20) unsigned NOT NULL,
  `nonce` BIGINT(20) unsigned NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE INDEX `block_miner_UNIQUE` (`block_height` ASC, `miner_id` ASC),
  INDEX `miner_fk_idx` (`miner_id` ASC)
)
ENGINE = InnoDB;