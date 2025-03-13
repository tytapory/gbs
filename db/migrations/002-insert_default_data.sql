INSERT INTO error_description(code, description)
VALUES (100, 'Transaction: Successful'),
       (101, 'Transaction: Sender does not exist'),
       (102, 'Transaction: Receiver does not exist'),
       (103, 'Transaction: Initiator does not exist'),
       (104, 'Transaction: Initiator is not the sender and does not have permission to manage funds'),
       (105, 'Transaction: Sender does not have "send_funds" permission'),
       (106, 'Transaction: Receiver does not have "receive_funds" permission'),
       (107, 'Transaction: Insufficient funds'),
       (108, 'Transaction: Amount less than or equal to zero'),
       (200, 'Print money: Successful'),
       (201, 'Print money: Receiver does not exist'),
       (202, 'Print money: Initiator does not exist'),
       (203, 'Print money: Initiator does not have permission to print money'),
       (204, 'Print money: Cant print values <= 0'),
       (300, 'Get balance: Successful'),
       (301, 'Get balance: Insufficient permissions'),
       (302, 'Get balance: User does not exists'),
       (401, 'Register: User already exists');
INSERT INTO permissions(name)
VALUES ('administrator'),
       ('manage_user_permissions'), --TODO
       ('manage_user_funds'),
       ('control_user_accounts'), --TODO
       ('print_money'),
       ('audit_funds'),
       ('receive_funds'),
       ('send_funds');

INSERT INTO users(username)
VALUES ('adm'), --1
       ('fees'), --2
       ('registration'), --3
       ('money_printer'); --4

INSERT INTO user_permission(user_id, permission_id)
VALUES (1, 1),
       (3, 4),
       (4, 5);
