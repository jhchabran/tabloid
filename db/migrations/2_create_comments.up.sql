CREATE TABLE comments (
	id serial PRIMARY KEY,
	parent_id integer NULL,
	score integer,
	body text,
	author varchar(255),
	created_at timestamp
);
