USE prototypes;

SET FOREIGN_KEY_CHECKS = 0;
TRUNCATE TABLE seats;
TRUNCATE TABLE users;
TRUNCATE TABLE trips;
SET FOREIGN_KEY_CHECKS = 1;

INSERT INTO trips (name) VALUES ('SOUTHWEST-101');

INSERT INTO users (name)
WITH RECURSIVE seq AS (
    SELECT 1 AS n
    UNION ALL
    SELECT n + 1 FROM seq WHERE n < 120
)
SELECT CONCAT(
    ELT(1 + ((n - 1) % 20),
        'Zoila', 'Andrew', 'Mia', 'Liam', 'Noah',
        'Ava', 'Ethan', 'Emma', 'Lucas', 'Olivia',
        'Mason', 'Sophia', 'Logan', 'Isabella', 'James',
        'Charlotte', 'Benjamin', 'Amelia', 'Elijah', 'Harper'
    ),
    ' ',
    ELT(1 + ((n - 1) % 20),
        'Rau', 'Miller', 'Davis', 'Wilson', 'Taylor',
        'Thomas', 'Garcia', 'Martinez', 'Anderson', 'Jackson',
        'White', 'Harris', 'Martin', 'Clark', 'Lewis',
        'Young', 'Hall', 'Allen', 'King', 'Wright'
    ),
    ' ',
    LPAD(n, 3, '0')
)
FROM seq;

INSERT INTO seats (name, trip_id, user_id)
WITH RECURSIVE seat_rows AS (
    SELECT 1 AS row_num
    UNION ALL
    SELECT row_num + 1 FROM seat_rows WHERE row_num < 20
),
seat_cols AS (
    SELECT 1 AS pos, 'A' AS col_name
    UNION ALL SELECT 2, 'B'
    UNION ALL SELECT 3, 'C'
    UNION ALL SELECT 4, 'D'
    UNION ALL SELECT 5, 'E'
    UNION ALL SELECT 6, 'F'
)
SELECT CONCAT(seat_rows.row_num, '-', seat_cols.col_name), 1, NULL
FROM seat_rows
CROSS JOIN seat_cols
ORDER BY seat_rows.row_num, seat_cols.pos;

SELECT
    (SELECT COUNT(*) FROM trips) AS trips_count,
    (SELECT COUNT(*) FROM users) AS users_count,
    (SELECT COUNT(*) FROM seats) AS seats_count;
