INSERT INTO permissions(name)
VALUES ('administrator'),
       ('manage_user_permissions'),
       ('manage_user_funds'),
       ('control_user_accounts'),
       ('print_money'),
       ('audit_funds'),
       ('receive_funds'),
       ('send_funds');

INSERT INTO users(username)
VALUES ('adm'), --1
       ('fees'); --2

INSERT INTO user_permission(user_id, permission_id)
VALUES (1, 1);