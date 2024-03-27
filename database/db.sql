CREATE TABLE IF NOT EXISTS "ims_registry"
(
    id             integer
    constraint ims_registry_pk
    primary key autoincrement,
    title          text,
    server_address text,
    username       text,
    password       text,
    email          text
);
CREATE TABLE ims_notice
(
    id         integer
        constraint ims_notice_pk
            primary key,
    type       text,
    title      text,
    message    text,
    created_at timestamp
);
CREATE TABLE IF NOT EXISTS "ims_event"
(
    id         integer
    constraint ims_event_pk
    primary key autoincrement,
    type       text,
    action     text,
    message    text,
    created_at text
);
CREATE TABLE IF NOT EXISTS "ims_image"
(
    id               integer
    constraint ims_image_pk
    primary key autoincrement,
    registry         text,
    tag              text,
    build_git        text,
    build_dockerfile text,
    build_zip        text,
    build_root       text,
    status           integer,
    message          text,
    build_type       text,
    build_template   text
    , image_info text);
CREATE TABLE IF NOT EXISTS "ims_site"
(
    id             integer
    constraint ims_site_pk
    primary key autoincrement,
    site_title     text,
    site_name      text,
    env            text,
    container_info text,
    status         integer,
    status_step    text,
    message        text,
    deleted_at     timestamp
);
CREATE TABLE ims_site_domain
(
    id           integer
        constraint ims_site_domain_pk
            primary key,
    container_id TEXT,
    server_name  text,
    port         integer
    , schema text, created_at timestamp);
CREATE TABLE IF NOT EXISTS "ims_setting"
(
    id         integer
    constraint ims_setting_pk
    primary key,
    group_name text,
    name       text,
    value      text
);
INSERT INTO ims_setting (group_name, name, value) VALUES ('user', 'founder', '{"password":"b9d11b3be25f5a1a7dc8ca04cd310b28","username":"admin"}');