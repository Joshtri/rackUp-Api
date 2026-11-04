//! RackUp API: billiard match tracking backend.
//!
//! Feature-based layout:
//! - `app`: state, router, health checks
//! - `cli`: admin commands (`cargo run -- user ...`)
//! - `config`: settings from env vars, logging
//! - `db`: connection pool, migrations
//! - `entities`: database rows and domain types
//! - `features`: one folder per feature (routes → handlers → service → repo)
//! - `middlewares`: auth extractor, CORS
//! - `shared`: errors, extractors, validation helpers

pub mod app;
pub mod cli;
pub mod config;
pub mod db;
pub mod entities;
pub mod features;
pub mod middlewares;
pub mod shared;
pub mod startup;
