CREATE TABLE comments (
	id serial PRIMARY KEY,
	story_id integer NULL,
	parent_comment_id integer NULL,
	upvotes integer,
	downvotes integer,
	body text,
	author_id integer NOT NULL,
	created_at timestamp NOT NULL
);
