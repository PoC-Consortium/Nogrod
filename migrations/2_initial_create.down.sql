START TRANSACTION;

CALL `proc_foreign_key_check`(
	'transaction',
    'account_fk',
    'ALTER TABLE `transaction` DROP FOREIGN KEY `account_fk`;',
    true);

CALL `proc_foreign_key_check`(
	'miner',
    'miner_account_fk',
    'ALTER TABLE `miner` DROP FOREIGN KEY `miner_account_fk`;',
    true);

CALL `proc_foreign_key_check`(
	'block',
    'nonce_submission_fk',
    'ALTER TABLE `block` DROP FOREIGN KEY `nonce_submission_fk`;',
    true);

CALL `proc_foreign_key_check`(
	'block',
    'winner_account_fk',
    'ALTER TABLE `block` DROP FOREIGN KEY `winner_account_fk`;',
    true);

CALL `proc_foreign_key_check`(
	'nonce_submission',
    'miner_block_fk',
    'ALTER TABLE `nonce_submission` DROP FOREIGN KEY `miner_block_fk`;',
    true);

CALL `proc_foreign_key_check`(
	'nonce_submission',
    'miner_fk',
    'ALTER TABLE `nonce_submission` DROP FOREIGN KEY `miner_fk`;',
    true);

DROP TABLE IF EXISTS `transaction`;
DROP TABLE IF EXISTS `miner`;
DROP TABLE IF EXISTS `block`;
DROP TABLE IF EXISTS `nonce_submission`;
DROP TABLE IF EXISTS `account`;
COMMIT;