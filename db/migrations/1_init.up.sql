CREATE TABLE stories (
	id serial PRIMARY KEY,
	title varchar(255),
	url varchar(255),
	body text,
	score integer,
	author_id integer NOT NULL,
	comments_count int DEFAULT 0,
	created_at timestamp NOT NULL
);
