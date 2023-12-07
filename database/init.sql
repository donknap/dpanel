create table ims_site
(
    id             integer
        constraint ims_site_pk
            primary key autoincrement,
    site_title     text,
    site_name      text,
    site_url       text,
    site_url_ext   text,
    env            text,
    type           integer,
    container_info text,
    status         integer,
    status_process integer,
    message        text,
    created_at     integer,
    deleted_at     timestamp

);

create table ims_image
(
    id          integer
        constraint ims_image_pk
            primary key autoincrement,
    name            text,
    md5             text,
    tag             text,
    size            text,
    build_git       text,
    registry        text,
    status          integer,
    status_step     text,
    message         text,
    created_at      integer,
    deleted_at      timestamp
);


create table ims_task
(
    id              integer
        constraint ims_task_pk
            primary key,
    task_id         integer,
    status          integer,
    message         text,
    step            text,
    type            text
);

