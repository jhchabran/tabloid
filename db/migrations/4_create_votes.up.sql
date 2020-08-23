CREATE TABLE votes (
	id serial PRIMARY KEY,
	comment_id integer default NULL,
	story_id integer default NULL,
	up boolean,
	user_id integer NOT NULL,
	created_at timestamp NOT NULL
);

CREATE INDEX votes_comment_id_idx ON votes (comment_id);
CREATE INDEX votes_story_id_idx ON votes (story_id);
CREATE INDEX votes_user_id_idx ON votes (user_id);
CREATE UNIQUE INDEX votes_stories_idx ON votes (user_id, story_id) WHERE comment_id IS NULL;
CREATE UNIQUE INDEX votes_comments_idx ON votes (user_id, comment_id) WHERE story_id IS NULL;

CREATE OR REPLACE FUNCTION update_stories_score() RETURNS TRIGGER AS $$
BEGIN
	UPDATE stories SET score = score + (case when NEW.up then 1 else -1 end) WHERE id=NEW.story_id;
	RETURN NULL; -- after trigger, result is ignored
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_stories_score
AFTER INSERT ON votes
FOR EACH ROW
	EXECUTE PROCEDURE update_stories_score();

CREATE OR REPLACE FUNCTION update_comments_score() RETURNS TRIGGER AS $$
BEGIN
	UPDATE comments SET score = score + (case when NEW.up then 1 else -1 end) WHERE id=NEW.comment_id;
	RETURN NULL; -- after trigger, result is ignored
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_comments_score
AFTER INSERT ON votes
FOR EACH ROW
	EXECUTE PROCEDURE update_comments_score();
