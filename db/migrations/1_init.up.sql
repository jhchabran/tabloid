CREATE TABLE stories (
	id serial PRIMARY KEY,
	title varchar(255),
	url varchar(255),
	body text,
	score integer,
	author_id integer NOT NULL,
	created_at timestamp NOT NULL
);
