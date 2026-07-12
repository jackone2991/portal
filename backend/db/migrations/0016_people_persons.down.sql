-- Reverse 0016_people_persons.
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code LIKE 'people:%'
);
DELETE FROM permissions WHERE code LIKE 'people:%';

DROP TABLE IF EXISTS people_birthday_notices;
DROP TABLE IF EXISTS people_persons;
