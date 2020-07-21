CREATE TABLE IF NOT EXISTS users (
	username   TEXT    PRIMARY KEY,
	passhash   BLOB    NOT NULL, -- bcrypt probably
	permission INTEGER NOT NULL, -- Permission enum
)

CREATE TABLE IF NOT EXISTS tokens (
	string    TEXT    NOT NULL,
	remaining INTEGER NOT NULL, -- (-1) for unlimited, owner only
)

CREATE TABLE IF NOT EXISTS posts (
	id         INTEGER PRIMARY KEY, -- Snowflake
	fileext    TEXT    NOT NULL,
	ownerid    INTEGER REFERENCES users(id) ON DELETE SET NULL,
	permission INTEGER NOT NULL, -- canAccess := users(perm) >= posts(perm)
)

CREATE TABLE IF NOT EXISTS posttags (
	postid  INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	tagname TEXT    NOT NULL,
)
