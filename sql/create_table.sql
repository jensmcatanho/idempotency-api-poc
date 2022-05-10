CREATE DATABASE idempotency;

CREATE TABLE IF NOT EXISTS users (
  id varchar(250) NOT NULL,
  email varchar(250) NOT NULL UNIQUE,
  password varchar(250) NOT NULL,
  PRIMARY KEY (id)
);