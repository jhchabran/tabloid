CREATE TABLE users (
	id serial PRIMARY KEY,
	name varchar(255),
	email varchar(255),
	created_at timestamp,
	last_login_at timestamp
);

CREATE UNIQUE INDEX users_name_idx ON users (name);
