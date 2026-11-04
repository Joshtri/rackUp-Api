//! Live shot-clock sessions shared between devices.
//!
//! The server stores the session state as the frontend sends it and only
//! guards the order of changes: every update must name the `version` it was
//! based on, so two phones tapping at once can't overwrite each other.
//! Phones poll `GET /live-sessions/{id}?since=<version>` to follow along.

pub mod dto;
pub mod handlers;
pub mod repo;
pub mod routes;
pub mod service;
