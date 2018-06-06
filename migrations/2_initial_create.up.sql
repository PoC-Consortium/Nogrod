START TRANSACTION;
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

CREATE TABLE IF NOT EXISTS `miner` (
  `id` BIGINT(20) unsigned NOT NULL,
  `capacity` BIGINT(20) NULL,
  PRIMARY KEY (`id`)
)
ENGINE = InnoDB;

CREATE TABLE IF NOT EXISTS `transaction` (
  `id` BIGINT(20) unsigned NOT NULL,
  `amount` BIGINT(20) NOT NULL,
  `recipient_id` BIGINT(20) unsigned NOT NULL,
  `created` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  INDEX `account_fk_idx` (`recipient_id` ASC)
)
ENGINE = InnoDB;

CALL `proc_foreign_key_check`(
	'nonce_submission',
  'miner_fk',
  '
ALTER TABLE `nonce_submission`
ADD CONSTRAINT `miner_fk`
    FOREIGN KEY (`miner_id`)
    REFERENCES `account` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
', false);

CALL `proc_foreign_key_check`(
	'nonce_submission',
  'miner_block_fk',
  '
ALTER TABLE `nonce_submission`
ADD CONSTRAINT `miner_block_fk`
    FOREIGN KEY (`block_height`)
    REFERENCES `block` (`height`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
', false);

CALL `proc_foreign_key_check`(
	'block',
  'winner_account_fk',
  '
ALTER TABLE `block`
ADD CONSTRAINT `winner_account_fk`
    FOREIGN KEY (`winner_id`)
    REFERENCES `account` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
', false);

CALL `proc_foreign_key_check`(
	'block',
  'nonce_submission_fk',
  '
ALTER TABLE `block`
ADD CONSTRAINT `nonce_submission_fk`
    FOREIGN KEY (`best_nonce_submission_id`)
    REFERENCES `nonce_submission` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
', false);

CALL `proc_foreign_key_check`(
	'miner',
    'miner_account_fk',
    '
ALTER TABLE `miner`
ADD CONSTRAINT `miner_account_fk`
    FOREIGN KEY (`id`)
    REFERENCES `account` (`id`)
    ON DELETE CASCADE
    ON UPDATE NO ACTION;
', false);

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