CREATE TABLE authors (
  id   BIGSERIAL PRIMARY KEY,
  name text      NOT NULL,
  bio  text
);

create table books (
  id BIGSERIAL PRIMARY KEY,
  name text NOT NULL,
  author BIGSERIAL references authors(id)
);
 
create view books_with_author as 
    select array_agg(books.name)::text[] as books, authors.* from books
    join authors on authors.id = books.author
    group by authors.id;