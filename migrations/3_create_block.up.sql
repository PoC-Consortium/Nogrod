CREATE TABLE IF NOT EXISTS `block` (
  `height` BIGINT(20) unsigned NOT NULL,
  `base_target` BIGINT(20) unsigned NOT NULL,
  `scoop` INT unsigned NOT NULL,
  `generation_signature` VARCHAR(64) NOT NULL,
  `winner_verified` TINYINT NOT NULL DEFAULT 0,
  `reward` BIGINT(20) NULL,
  `winner_id` BIGINT(20) unsigned NULL,
  `best_nonce_submission_id` BIGINT(20) NULL,
  `created` DATETIME NOT NULL,
  `generation_time` INT NOT NULL DEFAULT 240,
  PRIMARY KEY (`height`),
  INDEX `winner_account_fk_idx` (`winner_id` ASC),
  INDEX `nonce_submission_fk_idx` (`best_nonce_submission_id` ASC)
)
ENGINE = InnoDB;