CREATE TABLE stories (
	id serial PRIMARY KEY,
	title varchar(255),
	url varchar(255),
	body text,
	score integer default 0,
	author_id integer NOT NULL,
	comments_count integer DEFAULT 0,
	created_at timestamp NOT NULL
);
