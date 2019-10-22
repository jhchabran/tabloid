CREATE TABLE items (
	id serial PRIMARY KEY,
	title varchar(255),
	body text,
	score integer,
	author varchar(255),
	created_at timestamp
);
