CREATE TABLE comments (
	id serial PRIMARY KEY,
	story_id integer NULL,
	parent_comment_id integer NULL,
	score integer default 0,
	body text,
	author_id integer NOT NULL,
	created_at timestamp NOT NULL
);

CREATE OR REPLACE FUNCTION increment_comments_count() RETURNS TRIGGER AS $$
	BEGIN
		UPDATE stories SET comments_count = comments_count + 1 WHERE id=NEW.story_id;
	RETURN NULL; -- after trigger, result is ignored
	END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER increment_comments_count
	AFTER INSERT ON comments
	FOR EACH ROW
	EXECUTE PROCEDURE increment_comments_count();
