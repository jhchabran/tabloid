CREATE TABLE comments (
	id serial PRIMARY KEY,
	story_id integer NULL,
	parent_comment_id integer NULL,
	upvotes integer,
	downvotes integer,
	body text,
	author varchar(255),
	created_at timestamp
);
